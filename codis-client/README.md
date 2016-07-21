# Codis-Client
## ����
	codis-client��codis��Ⱥ�Ŀͻ���,֧��C++,Java,Python

## �������� (Python)
	1.redis-py-2.7.2
	2.hiredis(easy_install  hiredis)-0.2.0
	3.gcc-4.6.3
	4.jdk1.7
	5.python2.7
	6.kazoo
	
## Install java
	cd java
	mvn package
	���ɰ�Ϊ/target/bfdjodis-0.1.2-jar-with-dependencies.jar

## Install python
```python
	import BfdCodis
```

## C++ Demo
```c++
	#include "BfdCodis.h"
	
	//param[1]  zkAddr codis������zookeeper�ĵ�ַ
	//param[2]  proxyPath codis��zookeeper�е�proxy��path
	//param[3]  businessID ҵ�����
	BfdCodis pool("192.168.161.22:2181", "/zk/codis/db_test/proxy", "item");
	
	pool.set("kk", "vv");
	string value = pool.get("kk");
	
	�����ӳ�ʱ����
	int timeout = 50
	pool.set("kk","vv",timeout)
	ע��timeoutΪint�ͣ���λ���룬���ַ�ʽ�����ã��ڲ�����timeoutʱ��û�г�ʱʱ��
```

## Python Demo
```python
	import BfdCodis as codis
	
	//param[1]  zkAddr codis������zookeeper�ĵ�ַ
	//param[2]  proxyPath codis��zookeeper�е�proxy��path
	//param[3]  businessID ҵ�����
	client = codis.BfdCodis("192.168.161.22:2181", "/zk/codis/db_test23/proxy", "item")
	
	print client.set("key", "value")
	value = client.get("key")
```

## Java Demo
```Java 

	//param[1]  zkAddr codis������zookeeper�ĵ�ַ
	//param[2]	zkTimeout ����zookeeper�ĳ�ʱʱ��
	//param[3]  proxyPath codis��zookeeper�е�proxy��path
	//param[4]  config JedisPool��������Ϣ
	//param[5]  timeout  ��JedisPool��ȡ���ӵĳ�ʱʱ��
	//param[6]  businessID ҵ�����
	
	JedisPoolConfig config = new JedisPoolConfig();
	config.setMaxTotal(1000);
	config.setMaxIdle(1000);
	
	BfdJodis bfdjodis = new BfdJodis("192.168.161.22:2181", 3000, "/zk/codis/db_test23/proxy",
		config, 3000, "bfd");
    String str = bfdjodis.set("k1","v1");
	String str = bfdjodis.get("k1");
```	

