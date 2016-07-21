#Jodis - Java client for codis
Jodis is a java client for codis based on [Jedis](https://github.com/xetorthio/jedis) and [Curator](http://curator.apache.org/).

##Features
- Use a round robin policy to balance load to multiple codis proxies.
- Detect proxy online and offline automatically.


To use it
```java
BfdJodis bfdjodis = new BfdJodis("192.168.161.22:2181", 1000, "/zk/codis/db_test23/proxy",
			new JedisPoolConfig(), 1000, "bfd");
String str = bfdjodis.set("k1","v1");
```
Note: JDK7 is required to build and use jodis. If you want to use jodis with JDK6, you can copy the source files to your project, replace ThreadLocalRandom in BoundedExponentialBackoffRetryUntilElapsed and JDK7 specified grammar(maybe, not sure) , and then compile with JDK6.
