package codis

import akka.actor.ActorSystem
import codis.CodisClient._
import codis.CodisLogSource._
import org.apache.curator.framework.imps.CuratorFrameworkState
import org.apache.curator.framework.recipes.cache.PathChildrenCache.StartMode
import org.apache.curator.framework.recipes.cache.{PathChildrenCache, PathChildrenCacheEvent, PathChildrenCacheListener}
import org.apache.curator.framework.{CuratorFramework, CuratorFrameworkFactory}
import org.apache.curator.retry.BoundedExponentialBackoffRetry
import redis._
import redis.protocol.RedisReply
import spray.json.DefaultJsonProtocol._
import spray.json._

import scala.collection.JavaConversions._
import scala.concurrent.duration._
import scala.concurrent.{ExecutionContext, Future}

/**
 * A codis client based on {@code CuratorFramework} and {@code rediscala}.
 * Majority implementation are adopted from <a href="https://github.com/wandoulabs/codis/tree/master/extern/jodis">Jodis</a>.
 *
 * This implementation can be considered as two components.
 * One uses {@code ZooKeeper} to keep sync the list of proxy addresses by watching certain type of
 * events and update the list, if necessary.
 * If the list is updated, another component (implemented with {@link DynamicRedisClientPool})
 * immediately learn the difference, make adjustment and continue serve with latest update.
 *
 * By default, one actor will be created for each proxy, {@code PoolingConfig} can be used to alter
 * that number. Note that actor are dispatched sequentially, thus using multiple actors may increase
 * performance.
 *
 * @author Tianyi HE <tess3ract@wandoujia.com>
 */
class CodisClient(connectString: String,
                  zkPath: String,
                  config: PoolingConfig = new PoolingConfig(1),
                  sessionTimeout: Duration = 60 seconds,
                  connectTimeout: Duration = 15 seconds)
                 (implicit system: ActorSystem) extends RedisCommands {
  val log = akka.event.Logging(system, this)

  lazy val curatorClient = CuratorFrameworkFactory
    .builder()
    .connectString(connectString)
    .sessionTimeoutMs(sessionTimeout.toMillis.toInt)
    .connectionTimeoutMs(connectTimeout.toMillis.toInt)
    .retryPolicy(
      new BoundedExponentialBackoffRetry(CuratorRetryBaseSleep.toMillis.toInt,
                                         CuratorRetryMaxSleep.toMillis.toInt,
                                         CuratorMaxRetries))
    .build()

  lazy val watcher = new PathChildrenCache(curatorClient, zkPath, true)

  // bootstrap
  startWatcher
  log.info("Codis zookeeper watcher started.")

  // clientPool instance will serve as connection pool
  val clientPool = DynamicRedisClientPool(config.actorsEachProxy, reloadProxies)

  /**
   * Start watching configured {@code zkPath}, update list when change is notified.
   */
  def startWatcher() = {
    // we need to get the initial data so client must be started
    if (curatorClient.getState() == CuratorFrameworkState.LATENT) {
      curatorClient.start
    }
    watcher.start(StartMode.BUILD_INITIAL_CACHE)
    watcher.getListenable().addListener(new PathChildrenCacheListener() {
      override def childEvent(client: CuratorFramework, event: PathChildrenCacheEvent) = {
        if (CodisClient.ResetTypes.contains(event.getType())) {
          clientPool.update(reloadProxies)
        }
      }
    })
  }

  /**
   * Reload list of proxy from {@code ZooKeeper}, and keep {@code clientPool} synced.
   * @return
   */
  def reloadProxies(): Set[String] = {
    val children = collectionAsScalaIterable(watcher.getCurrentData)
    val proxies = readProxies(children.map(x => new String(x.getData)))
    log.info("Loaded proxies: {}", proxies)
    proxies.toSet
  }

  // forward methods
  override implicit val executionContext: ExecutionContext = clientPool.executionContext

  override def send[T](redisCommand: RedisCommand[_ <: RedisReply, T]): Future[T] =
    clientPool.send(redisCommand)
}

/**
 * Constants for watching and reading config
 */
object CodisClient {

  val ProxyAddressKey = "addr"

  val ProxyStateKey = "state"

  val ProxyStateOnline = "online"

  val CuratorRetryBaseSleep = 100 millis

  val CuratorRetryMaxSleep = 30 * 1000 millis

  val CuratorMaxRetries = -1

  val ResetTypes = Set(PathChildrenCacheEvent.Type.CHILD_ADDED,
                       PathChildrenCacheEvent.Type.CHILD_UPDATED,
                       PathChildrenCacheEvent.Type.CHILD_REMOVED)

  def readProxies(items: Iterable[String]) =
    items
      .map(_.parseJson.asJsObject)
      .filter(_.fields(CodisClient.ProxyStateKey).convertTo[String] == CodisClient.ProxyStateOnline)
      .map(_.fields(CodisClient.ProxyAddressKey).convertTo[String])

}

class PoolingConfig(val actorsEachProxy: Int)
