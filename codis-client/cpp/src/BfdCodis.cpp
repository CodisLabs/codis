#include "BfdCodis.h"

using namespace bfd::codis;

BfdCodis::BfdCodis(const string& zookeeperAddr, const string& proxyPath, const string& businessID)
{
	m_Pool = new RoundRobinCodisPool(zookeeperAddr, proxyPath, businessID);
}

BfdCodis::~BfdCodis()
{
	if (m_Pool = NULL)
	{
		delete m_Pool;
		m_Pool = NULL;
	}
}

bool BfdCodis::exists(string key, int tt)
{
	return m_Pool->GetProxy()->exists(key, tt);
}

int BfdCodis::del(string key, int tt)
{
	return m_Pool->GetProxy()->del(key, tt);
}

int BfdCodis::del(vector<string>& keys, int tt)
{
	return m_Pool->GetProxy()->del(keys, tt);
}
string BfdCodis::type(string key, int tt)
{
	return m_Pool->GetProxy()->type(key, tt);
}

bool BfdCodis::expire(string key, int seconds, int tt)
{
	return m_Pool->GetProxy()->expire(key, seconds, tt);
}

bool BfdCodis::set(string key, string value, int tt)
{
	return m_Pool->GetProxy()->set(key, value, tt);
}

bool BfdCodis::setnx(string key, string value, int tt)
{
	return m_Pool->GetProxy()->setnx(key, value, tt);
}

bool BfdCodis::setex(string key, string value, int seconds, int tt)
{
	return m_Pool->GetProxy()->setex(key, value, seconds, tt);
}

string BfdCodis::get(string key, int tt)
{
	return m_Pool->GetProxy()->get(key, tt);
}

string BfdCodis::getset(string key, string value, int tt)
{
	return m_Pool->GetProxy()->getset(key, value, tt);
}

int BfdCodis::setbit(string key, int index, bool value, int tt)
{
	return m_Pool->GetProxy()->setbit(key, index, value, tt);
}

int BfdCodis::getbit(string key, int index, int tt)
{
	return m_Pool->GetProxy()->getbit(key, index, tt);
}

int BfdCodis::bitcount(string key, int tt)
{
	return m_Pool->GetProxy()->bitcount(key, tt);
}

vector<string> BfdCodis::mget(vector<string>& keys, int tt)
{
	return m_Pool->GetProxy()->mget(keys, tt);
}

bool BfdCodis::mget2(vector<string>& keys, void (*callback)(KVMap& kvs))
{
	return m_Pool->GetProxy()->mget2(keys, callback);
}

bool BfdCodis::mset(map<string, string>& keyvalues, int tt)
{
	return m_Pool->GetProxy()->mset(keyvalues, tt);
}

int BfdCodis::incr(string key, int tt)
{
	return m_Pool->GetProxy()->incr(key, tt);
}

int BfdCodis::decr(string key, int tt)
{
	return m_Pool->GetProxy()->decr(key, tt);
}

int BfdCodis::incrby(string key, int incr, int tt)
{
	return m_Pool->GetProxy()->incrby(key, incr, tt);
}

int BfdCodis::decrby(string key, int decr, int tt)
{
	return m_Pool->GetProxy()->decrby(key, decr, tt);
}

long BfdCodis::append(string key, string value, int tt)
{
	return m_Pool->GetProxy()->append(key, value, tt);
}

int BfdCodis::lpush(string key, string value, int tt)
{
	return m_Pool->GetProxy()->lpush(key, value, tt);
}

int BfdCodis::rpush(string key, string value, int tt)
{
	return m_Pool->GetProxy()->rpush(key, value, tt);
}

int BfdCodis::lpush(string key, vector<string> values, int tt)
{
	return m_Pool->GetProxy()->lpush(key, values, tt);
}

int BfdCodis::rpush(string key, vector<string> values, int tt)
{
	return m_Pool->GetProxy()->rpush(key, values, tt);
}

int BfdCodis::llen(string key, int tt)
{
	return m_Pool->GetProxy()->llen(key, tt);
}

vector<string> BfdCodis::lrange(string key, int start, int end, int tt)
{
	return m_Pool->GetProxy()->lrange(key, start, end, tt);
}

bool BfdCodis::ltrim(string key, int start, int end, int tt)
{
	return m_Pool->GetProxy()->ltrim(key, start, end, tt);
}

bool BfdCodis::lset(string key, int index, string value, int tt)
{
	return m_Pool->GetProxy()->lset(key, index, value, tt);
}

bool BfdCodis::lrem(string key, int count, string value, int tt)
{
	return m_Pool->GetProxy()->lrem(key, count, value, tt);
}

string BfdCodis::lpop(string key, int tt)
{
	return m_Pool->GetProxy()->lpop(key, tt);
}

string BfdCodis::rpop(string key, int tt)
{
	return m_Pool->GetProxy()->rpop(key, tt);
}

bool BfdCodis::sadd(string key, string member, int tt)
{
	return m_Pool->GetProxy()->sadd(key, member, tt);
}

int BfdCodis::sadd(string key, vector<string> members, int tt)
{
	return m_Pool->GetProxy()->sadd(key, members, tt);
}

bool BfdCodis::srem(string key, string member, int tt)
{
	return m_Pool->GetProxy()->srem(key, member, tt);
}

string BfdCodis::spop(string key, int tt)
{
	return m_Pool->GetProxy()->spop(key, tt);
}

string BfdCodis::srandmember(string key, int tt)
{
	return m_Pool->GetProxy()->srandmember(key, tt);
}

int BfdCodis::scard(string key, int tt)
{
	return m_Pool->GetProxy()->scard(key, tt);
}

bool BfdCodis::sismember(string key, string member, int tt)
{
	return m_Pool->GetProxy()->sismember(key, member, tt);
}

vector<string> BfdCodis::smembers(string key, int tt)
{
	return m_Pool->GetProxy()->smembers(key, tt);
}

bool BfdCodis::zadd(string key, int score, string member, int tt)
{
	return m_Pool->GetProxy()->zadd(key, score, member, tt);
}

bool BfdCodis::zrem(string key, string member, int tt)
{
	return m_Pool->GetProxy()->zrem(key, member, tt);
}

int BfdCodis::zincrby(string key, int incr, string member, int tt)
{
	return m_Pool->GetProxy()->zincrby(key, incr, member, tt);
}

int BfdCodis::zrank(string key, string member, int tt)
{
	return m_Pool->GetProxy()->zrank(key, member, tt);
}

int BfdCodis::zrevrank(string key, string member, int tt)
{
	return m_Pool->GetProxy()->zrevrank(key, member, tt);
}

vector<string> BfdCodis::zrange(string key, int start, int end, string withscores, int tt)
{
        return m_Pool->GetProxy()->zrange(key, start, end, withscores, tt);
}

vector<string> BfdCodis::zrevrange(string key, int start, int end, string withscores, int tt)
{
        return m_Pool->GetProxy()->zrevrange(key, start, end, withscores, tt);
}

vector<string> BfdCodis::zrangebyscore(string key, string min, string max, string withscores, int tt)
{
        return m_Pool->GetProxy()->zrangebyscore(key, min, max, withscores, tt);
}

vector<string> BfdCodis::zrevrangebyscore(string key, string min, string max, string withscores, int tt)
{
        return m_Pool->GetProxy()->zrevrangebyscore(key, min, max, withscores, tt);
}

int BfdCodis::zcount(string key, int min, int max, int tt)
{
	return m_Pool->GetProxy()->zcount(key, min, max, tt);
}

int BfdCodis::zcard(string key, int tt)
{
	return m_Pool->GetProxy()->zcard(key, tt);
}

int BfdCodis::zscore(string key, string member, int tt)
{
	return m_Pool->GetProxy()->zscore(key, member, tt);
}

int BfdCodis::zremrangebyrank(string key, int min, int max, int tt)
{
	return m_Pool->GetProxy()->zremrangebyrank(key, min, max, tt);
}

int BfdCodis::zremrangebyscore(string key, int min, int max, int tt)
{
	return m_Pool->GetProxy()->zremrangebyscore(key, min, max, tt);
}

bool BfdCodis::hset(string key, string field, string value, int tt)
{
	return m_Pool->GetProxy()->hset(key, field, value, tt);
}

string BfdCodis::hget(string key, string field, int tt)
{
	return m_Pool->GetProxy()->hget(key, field, tt);
}

vector<string> BfdCodis::hmget(string key, vector<string>& field, int tt)
{
	return m_Pool->GetProxy()->hmget(key, field, tt);
}

bool BfdCodis::hmset(string key, vector<string>& fields, vector<string>& values, int tt)
{
	return m_Pool->GetProxy()->hmset(key, fields, values, tt);
}

int BfdCodis::hincrby(string key, string field, int incr, int tt)
{
	return m_Pool->GetProxy()->hincrby(key, field, incr, tt);
}

bool BfdCodis::hexists(string key, string field, int tt)
{
	return m_Pool->GetProxy()->hexists(key, field, tt);
}

bool BfdCodis::hdel(string key, string field, int tt)
{
	return m_Pool->GetProxy()->hdel(key, field, tt);
}

int BfdCodis::hlen(string key, int tt)
{
	return m_Pool->GetProxy()->hlen(key, tt);
}

vector<string> BfdCodis::hkeys(string key, int tt)
{
	return m_Pool->GetProxy()->hkeys(key, tt);
}

vector<string> BfdCodis::hvals(string key, int tt)
{
	return m_Pool->GetProxy()->hvals(key, tt);
}

bool BfdCodis::hgetall(string key, vector<string>& fields, vector<string>& values, int tt)
{
	return m_Pool->GetProxy()->hgetall(key, fields, values, tt);
}

Reply BfdCodis::RedisCommand(const vector<string>& command, int tt)
{
	return m_Pool->GetProxy()->RedisCommand(command, tt);
}

Reply BfdCodis::RedisCommand(Command& command, int tt)
{
	return m_Pool->GetProxy()->RedisCommand(command, tt);
}

vector<Reply> BfdCodis::RedisCommands(vector<Command>& commands)
{
	return m_Pool->GetProxy()->RedisCommands(commands);
}

