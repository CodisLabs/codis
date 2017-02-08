package codis

import org.scalatest.{Matchers, WordSpecLike}

/**
 * @author Tianyi HE <tess3ract@wandoujia.com>
 */
class CodisClientSpec extends WordSpecLike with Matchers {

  "codis client" must {
    "parse proxies correctly" in {
      val proxies = CodisClient.readProxies( """{"addr":"127.0.0.1:6379","state":"online"}""" ::
                                               """{"addr":"127.0.0.1:6380","state":"online"}""" :: Nil)
      proxies.size should equal(2)
      proxies.toSeq(0) should equal("127.0.0.1:6379")
      proxies.toSeq(1) should equal("127.0.0.1:6380")
    }
    "ignore non-online proxies" in {
      val proxies = CodisClient.readProxies( """{"addr":"127.0.0.1:6379","state":"online"}""" ::
                                               """{"addr":"127.0.0.1:6380","state":"offline"}""" :: Nil)
      proxies.size should equal(1)
      proxies.toSeq(0) should equal("127.0.0.1:6379")
    }
  }

}
