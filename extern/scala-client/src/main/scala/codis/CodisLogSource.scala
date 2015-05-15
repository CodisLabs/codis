package codis

import akka.event.LogSource

/**
 * Use as akka log source for codis client.
 *
 * @author Tianyi HE <tess3ract@wandoujia.com>
 */
object CodisLogSource {

  implicit val logSource: LogSource[AnyRef] = new LogSource[AnyRef] {
    def genString(o: AnyRef): String = o.getClass.getName

    override def getClazz(o: AnyRef): Class[_] = o.getClass
  }

}
