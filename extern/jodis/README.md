#Jodis - Java client for codis
Jodis is a java client for codis based on [Jedis](https://github.com/xetorthio/jedis) and [Curator](http://curator.apache.org/).

##Features
- Use a round robin policy to balance load to multiple codis proxies.
- Automatic new proxy and offline proxy detection.

##How to use
Add this to your pom.xml. We deploy jodis to https://oss.sonatype.org.
```xml
<dependency>
  <groupId>com.wandoulabs.jodis</groupId>
  <artifactId>jodis</artifactId>
  <version>0.1.1</version>
</dependency>
```
To use it
```java
JedisResourcePool jedisPool = new RoundRobinJedisPool("zkserver:2181", 30000, "/zk/codis/db_xxx/proxy", new JedisPoolConfig());
try (Jedis jedis = jedisPool.getResource()) {
    jedis.set("foo", "bar");
    String value = jedis.get("foo");
}
```
Note: JDK7 is required to build jodis. I think JDK6 is enough to run jodis but I haven't tested it. So I recommend to use jodis with JDK7.