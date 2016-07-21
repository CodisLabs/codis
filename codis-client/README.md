# Codis-Client
## 介绍
	codis-client是codis集群的客户端,支持C++,Java,Python

## 环境依赖 (Python)
	1.redis-py-2.7.2
	2.hiredis(easy_install  hiredis)-0.2.0
	3.gcc-4.6.3
	4.jdk1.7
	5.python2.7
	6.kazoo
	
## Install java
	cd java
	mvn package
	生成包为/target/bfdjodis-0.1.2-jar-with-dependencies.jar

## Install python
```python
	import BfdCodis
```

## C++ Demo
```c++
	#include "BfdCodis.h"
	
	//param[1]  zkAddr codis依赖的zookeeper的地址
	//param[2]  proxyPath codis在zookeeper中的proxy的path
	//param[3]  businessID 业务分类
	BfdCodis pool("192.168.161.22:2181", "/zk/codis/db_test/proxy", "item");
	
	pool.set("kk", "vv");
	string value = pool.get("kk");
	
	新增加超时机制
	int timeout = 50
	pool.set("kk","vv",timeout)
	注：timeout为int型，单位毫秒，两种方式均可用，在不传入timeout时，没有超时时间
```

## Python Demo
```python
	import BfdCodis as codis
	
	//param[1]  zkAddr codis依赖的zookeeper的地址
	//param[2]  proxyPath codis在zookeeper中的proxy的path
	//param[3]  businessID 业务分类
	client = codis.BfdCodis("192.168.161.22:2181", "/zk/codis/db_test23/proxy", "item")
	
	print client.set("key", "value")
	value = client.get("key")
```

## Java Demo
```Java 

	//param[1]  zkAddr codis依赖的zookeeper的地址
	//param[2]	zkTimeout 连接zookeeper的超时时间
	//param[3]  proxyPath codis在zookeeper中的proxy的path
	//param[4]  config JedisPool的配置信息
	//param[5]  timeout  从JedisPool获取连接的超时时间
	//param[6]  businessID 业务分类
	
	JedisPoolConfig config = new JedisPoolConfig();
	config.setMaxTotal(1000);
	config.setMaxIdle(1000);
	
	BfdJodis bfdjodis = new BfdJodis("192.168.161.22:2181", 3000, "/zk/codis/db_test23/proxy",
		config, 3000, "bfd");
    String str = bfdjodis.set("k1","v1");
	String str = bfdjodis.get("k1");
```	

