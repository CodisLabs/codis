package redis

import akka.actor.{Actor, ActorRef, ActorSystem, Props}
import akka.pattern._
import akka.testkit.{ImplicitSender, TestKit}
import akka.util.Timeout
import org.scalatest.{Matchers, WordSpecLike}
import redis.DynamicRedisClientPool.ConnectOperations
import redis.FakeRedisClientActor.GetServer

import scala.concurrent.Await
import scala.concurrent.duration._

/**
 * @author Tianyi HE <tess3ract@wandoujia.com>
 */
class DynamicRedisClientPoolSpec
  extends TestKit(ActorSystem("testActorSystem"))
  with ImplicitSender
  with WordSpecLike
  with Matchers {

  implicit val timeout = Timeout(3 seconds)

  def checkNext(host:String, port:Int) =
    Await.result(pool.getNextConnection ? GetServer, timeout.duration) match {
      case RedisServer(actualHost, actualPort, _, _) => {
        println(s"host: $host, port: $port")
        actualHost should equal(host)
        actualPort should equal(port)
      }
    }

  val pool = DynamicRedisClientPool(2, Set("host0:6379", "host1:6379"))(system,
                                                                        FakeRedisClientActor.apply)

  "round robin" must {
    "work with initial state" in {
      pool.redisServers.size should equal(2)
      // actorsEachProxy * 2, where actorsEachProxy = 2
      pool.redisConnectionPool.size should equal(4)
      pool.connectionGroups.size should equal(2)
      for (i <- 0 to 5) {
        // make sure hosts are arranged in interleaved round-robin fashion
        checkNext("host0", 6379)
        checkNext("host1", 6379)
      }
    }
    "work after server added" in {
      pool.update(Set("host0:6379", "host1:6379", "host2:6379"))
      pool.redisServers.size should equal(3)
      pool.redisConnectionPool.size should equal(6)
      pool.connectionGroups.size should equal(3)
      for (i <- 0 to 5) {
        checkNext("host0", 6379)
        checkNext("host1", 6379)
        checkNext("host2", 6379)
      }
    }
    "work after server order changed" in {
      pool.update(Set("host0:6379", "host2:6379", "host1:6379"))
      pool.redisServers.size should equal(3)
      pool.redisConnectionPool.size should equal(6)
      pool.connectionGroups.size should equal(3)
      for (i <- 0 to 5) {
        // order are preserved after first seen, thus this will not affect the original order (0-1-2)
        checkNext("host0", 6379)
        checkNext("host1", 6379)
        checkNext("host2", 6379)
      }
    }
    "work after server removed" in {
      pool.update(Set("host0:6379", "host2:6379"))
      pool.redisServers.size should equal(2)
      pool.redisConnectionPool.size should equal(4)
      pool.connectionGroups.size should equal(2)
      for (i <- 0 to 5) {
        checkNext("host0", 6379)
        checkNext("host2", 6379)
      }
    }
  }
}

object FakeRedisClientActor {
  case object GetServer
  def apply(system: ActorSystem, server: RedisServer): ActorRef =
    system.actorOf(Props(classOf[FakeRedisClientActor], server))
}

class FakeRedisClientActor(server: RedisServer) extends Actor {

  override def receive: Receive = {
    case GetServer => sender ! server
    case x => println(s"Received: $x")
  }
}
