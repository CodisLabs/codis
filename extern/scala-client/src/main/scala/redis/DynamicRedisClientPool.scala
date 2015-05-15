package redis

import java.net.InetSocketAddress

import akka.actor.{ActorRef, ActorSystem, Props}
import codis.CodisLogSource._
import redis.DynamicRedisClientPool._
import redis.actors.RedisClientActor

import scala.concurrent.ExecutionContext

/**
 * A dynamic redis pool implementation that supports proxy list update and round-robin.
 * The implementation is similar to {@link redis.RedisClientPool} except that {@code redisServers}
 * become mutable and supports differential update.
 *
 * Initially, the behavior of this implementation is nearly same with other round-robin pool, with
 * the difference that it creates one or more connections per server, and store connections grouped
 * by server. This allows operating on multiple connection easily.
 * When {@code update} get called, it compares the list with current one, then perform operations,
 * namely add and remove. These operations will be performed on {@code connectionGroups}.
 * After these operations, new connections are appended, deprecated connections are removed. It is
 * then flattened and stored in {@code redisConnectionPool} for real round-robin.
 *
 * The behavior of round robin is specified in {@link RoundRobinPoolRequest}.
 *
 * @see redis.RedisClientPool
 * @author Tianyi HE <tess3ract@wandoujia.com>
 */
case class DynamicRedisClientPool(actorsEachProxy: Int, var proxies: Set[String])
                                 (implicit system: ActorSystem,
                                  clientActorSupplier: ClientSupplier = clientActor)
  extends RoundRobinPoolRequest with RedisCommands {

  // groups of connections, each group represent a set of connections to identical redis instance
  // the size of each group is specified by [actorsEachProxy]
  type ConnectionGroups = Map[RedisServer, Seq[ActorRef]]

  val log              = akka.event.Logging(system, this)
  val executionContext = system.dispatcher

  var redisServers       : Set[RedisServer] = Set.empty
  var redisConnectionPool: Seq[ActorRef]    = Nil
  var connectionGroups   : ConnectionGroups = Map.empty

  // bootstrap
  redisServers = proxies.map(addr2server)
  refreshConnections

  /**
   * Create a group of connections to given {@code server}.
   * Number of connections is specified by {@code actorsEachProxy}.
   *
   * @param server
   * @return connection actors
   */
  def connectionGroup(server: RedisServer): Seq[ActorRef] = {
    log.info("Creating connection group for {}", server)
    (0 until actorsEachProxy).map(_ => clientActorSupplier(system, server))
  }

  /**
   * Refresh created connections and connection pool with latest {@code redisServers}.
   * The method compares {@code redisServers} with existing {@code connectionGroups}, then perform
   * alter operations when adds or removes are detected.
   *
   * Operations are performed on {@code connectionGroups}, then it is flattened and assigned to
   * {@code redisConnectionPool} for round-robin (consider as a snapshot which is immutable in period).
   */
  def refreshConnections() = {
    log.info("Refreshing connections using server list: {}", redisServers)
    // server in redisServers but does not exist in connectionGroups is considered added
    val added = redisServers.filter(!connectionGroups.contains(_))
    // server in connectionGroups but does not exist in redisServers is considered removed
    val removed = connectionGroups.filter(kv => !redisServers.contains(kv._1))
    val retained = connectionGroups -- removed.keys
    log.info("Added: {}", added)
    log.info("Removed: {}", removed.keys)
    connectionGroups = retained ++ added.map(server => (server -> connectionGroup(server))).toMap
    // flatten connection groups, make sure it is interleaved (i.e. 0-1-2-0-1-2, instead of 0-0-1-1-2-2)
    val matrix = connectionGroups.values.transpose
    redisConnectionPool = matrix.flatten.toSeq
    // kill removed connections, this may cause ongoing async redis operations fail
    removed.values.flatten.foreach(system.stop)
  }

  /**
   * Update the list of proxies.
   *
   * @param proxies
   */
  def update(proxies: Set[String]) = {
    redisServers = proxies.map(addr2server)
    refreshConnections
  }

  /**
   * Get one connection due to round-robin.
   * Immutable snapshot of connection pool are first retrieved for thread-safety.
   * @return
   */
  override def getNextConnection: ActorRef = {
    // read redisServers only once to prevent inconsistency
    val currentConnectionPool = redisConnectionPool
    if (currentConnectionPool.size == 0)
      throw new IllegalStateException("No available redis server.")
    currentConnectionPool(next.getAndIncrement % currentConnectionPool.size)
  }

  /**
   * Disconnect from the server (stop the actor)
   */
  def stop() = redisConnectionPool.foreach(system.stop)

}

object DynamicRedisClientPool {

  type ConnectOperations = () => Seq[Operation[_, _]]
  type ClientSupplier = (ActorSystem, RedisServer) => ActorRef

  implicit def addr2server(addr: String): RedisServer = {
    val components = addr.split(":")
    RedisServer(components(0), components(1).toInt)
  }

  def clientActor(system: ActorSystem, server: RedisServer) =
    system.actorOf(Props(classOf[RedisClientActor],
                         new InetSocketAddress(server.host, server.port),
                         getConnectOperations(server)(system)).withDispatcher(Redis.dispatcher),
                   "codis-" + Redis.tempName())

  // method adopted from redis.RedisClientPool
  def getConnectOperations(server: RedisServer)(implicit system: ActorSystem): ConnectOperations =
    () => {
      val redis = new BufferedRequest with RedisCommands {
        implicit val executionContext: ExecutionContext = system.dispatcher
      }
      redis.operations.result()
    }

}
