#include "CodisClient.h"
#include "RoundRobinCodisPool.h"
#include "gtest/gtest.h"
#include "Utils.h"


using namespace std;
using namespace bfd::codis;

CodisClient client("192.168.161.20", 6379, "item");


TEST(RedisClientTest, setRedisCommand)
{

	vector<string> command;
	command.push_back("set");
	command.push_back("kk");
	command.push_back("vv");
	Reply ret = client.RedisCommand(command);
	EXPECT_STREQ(ret.str().c_str(), "OK");
}

TEST(RedisClientTest, setCommand)
{
	Reply ret = client.RedisCommand(Command("SET")("kk")("vv"));
	EXPECT_STREQ(ret.str().c_str(), "OK");
}

TEST(RedisClientTest, getRedisCommand)
{
	vector<string> command;
	command.push_back("get");
	command.push_back("kk");
	Reply ret = client.RedisCommand(command);
	EXPECT_STREQ(ret.str().c_str(), "vv");
}

TEST(RedisClientTest, RedisCommands)
{

	vector<Command> commands;
	Command comm("get");
	comm("kk");
	commands.push_back(comm);

	vector<Reply> ret = client.RedisCommands(commands);
	EXPECT_EQ(ret.size(), 1);
	EXPECT_STREQ(ret[0].str().c_str(), "vv");
}

TEST(RedisClientTest, getCommand)
{
	Reply ret = client.RedisCommand(Command("GET")("kk"));
	EXPECT_STREQ(ret.str().c_str(), "vv");
}


TEST(RedisClientTest, type)
{
	string ret = client.type("kk");
	EXPECT_STREQ(ret.c_str(), "string");
}

TEST(RedisClientTest, expire)
{
	bool ret = client.expire("kk", 100);
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, del)
{
	int ret = client.del("kk");
	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, set)
{
	bool ret = client.set("kk", "vv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, setnx_exist)
{
	bool ret = client.setnx("kk", "vv");
	EXPECT_TRUE(!ret);
}

TEST(RedisClientTest, setnx)
{
	client.del("aaa");
	bool ret = client.setnx("aaa", "bbb");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, setex)
{
	client.del("aaa");
	bool ret = client.setex("aaa", "bbb", 100);
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, get)
{
	string ret = client.get("kk");
	EXPECT_STREQ(ret.c_str(), "vv");
}

TEST(RedisClientTest, getset)
{
	string ret = client.getset("kk", "vvv");
	EXPECT_STREQ(ret.c_str(), "vv");
}

TEST(RedisClientTest, mset)
{
	map<string, string> kvs;
	for (int i=1; i<5; i++)
	{
		kvs[string("aa") + int2string(i)] = string("bb") + int2string(i);
	}

	bool ret = client.mset(kvs);
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, mget)
{
	vector<string> keys;
	for (int i=1; i<5; i++)
	{
		string key;
		key = string("aa") + int2string(i);
		keys.push_back(key);
	}

	vector<string> ret = client.mget(keys);

	for (int i=1; i<5; i++)
	{
		EXPECT_STREQ(ret[i-1].c_str(), (string("bb") + int2string(i)).c_str());
	}
}

void callback(map<string, string>& kvmap)
{
	cout << "callllback............" << endl;
}

TEST(RedisClientTest, mget2)
{
	vector<string> keys;
	for (int i=1; i<5; i++)
	{
		string key;
		key = string("aa") + int2string(i);
		keys.push_back(key);
	}

	bool ret = client.mget2(keys, callback);

	EXPECT_TRUE(ret);

}



TEST(RedisClientTest, exists)
{
	bool ret = client.exists("kk");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, incr)
{
	client.del("num");
	int ret = client.incr("num");
	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, decr)
{
	int ret = client.decr("num");
	EXPECT_EQ(ret, 0);
}

TEST(RedisClientTest, incrby)
{
	int ret = client.incrby("num", 1);
	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, decrby)
{
	int ret = client.decrby("num", 1);
	EXPECT_EQ(ret, 0);
}

TEST(RedisClientTest, append)
{
	long ret = client.append("kk", "vv");
	EXPECT_EQ(ret, 5);
}


TEST(RedisClientTest, lpush)
{
	client.del("list");
	int ret = client.lpush("list", "lv");
	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, rpush)
{
	int ret = client.rpush("list", "rv");
	EXPECT_EQ(ret, 2);
}

TEST(RedisClientTest, llen)
{
	int ret = client.llen("list");
	EXPECT_EQ(ret, 2);
}


TEST(RedisClientTest, lrange)
{
	vector<string> ret = client.lrange("list", 0, 1);
	EXPECT_EQ(ret.size(), 2);
	EXPECT_STREQ(ret[0].c_str(), "lv");
	EXPECT_STREQ(ret[1].c_str(), "rv");
}

TEST(RedisClientTest, ltrim)
{
	bool ret = client.ltrim("list", 0, 1);
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, lset)
{
	bool ret = client.lset("list", 0, "lv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, lrem)
{
	bool ret = client.lrem("list", 1, "lv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, lpop)
{
	string ret = client.lpop("list");
	EXPECT_STREQ(ret.c_str(), "rv");
}

TEST(RedisClientTest, rpop)
{
	client.lpush("list", "rv");
	string ret = client.rpop("list");
	EXPECT_STREQ(ret.c_str(), "rv");
}

TEST(RedisClientTest, sadd)
{
	client.srem("sadd", "saddv");
	bool ret = client.sadd("sadd", "saddv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, srem)
{
	bool ret = client.srem("sadd", "saddv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, spop)
{
	client.sadd("sadd", "saddv");
	string ret = client.spop("sadd");
	EXPECT_STREQ(ret.c_str(), "saddv");
}

TEST(RedisClientTest, srandmember)
{
	client.sadd("sadd", "saddv");
	string ret = client.srandmember("sadd");
	EXPECT_STREQ(ret.c_str(), "saddv");
}

TEST(RedisClientTest, scard)
{
	int ret = client.scard("sadd");
	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, sismember)
{
	bool ret = client.sismember("sadd", "saddv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, smembers)
{
	vector<string> ret = client.smembers("sadd");
	EXPECT_EQ(ret.size(), 1);
	EXPECT_STREQ(ret[0].c_str(), "saddv");
}

TEST(RedisClientTest, zadd)
{
	client.zrem("zadd", "zaddv");
	bool ret = client.zadd("zadd", 1, "zaddv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, zrem)
{
	bool ret = client.zrem("zadd", "zaddv");
	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, zincrby)
{
	client.zadd("zadd", 1, "zaddv");
	int ret = client.zincrby("zadd", 2, "zaddv");
	EXPECT_EQ(ret, 3);
}

TEST(RedisClientTest, zrank)
{
	client.zadd("zadd", 2, "zaddv2");
	int ret = client.zrank("zadd", "zaddv2");
	EXPECT_EQ(ret, 0);
}

TEST(RedisClientTest, zrevrank)
{
	int ret = client.zrevrank("zadd", "zaddv2");
	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, zrange)
{
	vector<string> ret = client.zrange("zadd", 0, 1);

	EXPECT_EQ(ret.size(), 2);
}

TEST(RedisClientTest, zrevrange)
{
	vector<string> ret = client.zrevrange("zadd", 0, 1);

	EXPECT_EQ(ret.size(), 2);
}

TEST(RedisClientTest, zrangebyscore)
{
	vector<string> ret = client.zrangebyscore("zadd", 2, 3);

	EXPECT_EQ(ret.size(), 2);
}

TEST(RedisClientTest, zcount)
{
	int ret = client.zcount("zadd", 2, 3);

	EXPECT_EQ(ret, 2);
}

TEST(RedisClientTest, zcard)
{
	int ret = client.zcard("zadd");

	EXPECT_EQ(ret, 2);
}

TEST(RedisClientTest, zscore)
{
	int ret = client.zscore("zadd", "zaddv2");

	EXPECT_EQ(ret, 2);
}

TEST(RedisClientTest, zremrangebyrank)
{
	int ret = client.zremrangebyrank("zadd", 0, 1);

	EXPECT_EQ(ret, 2);
}

TEST(RedisClientTest, zremrangebyscore)
{
	client.zadd("zadd", 2, "zaddv2");
	client.zadd("zadd", 3, "zaddv3");
	int ret = client.zremrangebyscore("zadd", 2, 3);

	EXPECT_EQ(ret, 2);
}

TEST(RedisClientTest, hset)
{
	bool ret = client.hset("hset", "field1", "value1");

	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, hget)
{
	string ret = client.hget("hset", "field1");

	EXPECT_STREQ(ret.c_str(), "value1");
}

TEST(RedisClientTest, hmget)
{
	vector<string> fields;
	fields.push_back("field1");
	vector<string> ret = client.hmget("hset", fields);

	EXPECT_EQ(ret.size(), 1);

	EXPECT_STREQ(ret[0].c_str(), "value1");
}

TEST(RedisClientTest, hincrby)
{
	client.hset("hset", "field2", "11");
	int ret = client.hincrby("hset", "field2", 2);

	EXPECT_EQ(ret, 13);
}

TEST(RedisClientTest, hexists)
{
	bool ret = client.hexists("hset", "field1");

	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, hdel)
{
	bool ret = client.hdel("hset", "field1");

	EXPECT_TRUE(ret);
}

TEST(RedisClientTest, hlen)
{
	int ret = client.hlen("hset");

	EXPECT_EQ(ret, 1);
}

TEST(RedisClientTest, hkeys)
{
	vector<string> ret = client.hkeys("hset");

	EXPECT_EQ(ret.size(), 1);
	EXPECT_STREQ(ret[0].c_str(), "field2");
}

TEST(RedisClientTest, hvals)
{
	vector<string> ret = client.hvals("hset");

	EXPECT_EQ(ret.size(), 1);
	EXPECT_STREQ(ret[0].c_str(), "13");
}

TEST(RedisClientTest, hgetall)
{
	vector<string> keys, values;
	bool ret = client.hgetall("hset", keys, values);
	EXPECT_TRUE(ret);
	EXPECT_EQ(keys.size(), 1);
	EXPECT_EQ(values.size(), 1);
	EXPECT_STREQ(keys[0].c_str(), "field2");
	EXPECT_STREQ(values[0].c_str(), "13");
}

TEST(RedisClientTest, codis)
{
	RoundRobinCodisPool pool("192.168.161.22:2181", "/zk/codis/db_test/proxy", "businessID");

	for (int i=0; i<10; i++)
	{
		bool ret = pool.GetProxy()->set("kkk", "vvv");
		cout << "ret=" << ret << endl;
		sleep(1);
	}
}
