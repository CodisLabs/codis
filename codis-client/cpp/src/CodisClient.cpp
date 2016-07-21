#include "CodisClient.h"
#include "Utils.h"
#include "Log.h"
#include <exception>
#include <assert.h>

using namespace bfd::codis;

struct AsyncInfo
{
//	RedisDB db;
//	vector<string> keys;
//	MgetAsyncRequestContext2 mget_async_rctxt;
};





CodisClient::CodisClient(const string& proxyIP, const int port, const string& businessID)
{
	m_ConnPool = new RedisClientPool(proxyIP, port);

	m_BID = businessID;

	m_Loop = aeCreateEventLoop(64);

	pthread_t AEThreadID;
    
    proxy_IP = proxyIP;

    proxy_Port = port;

	int ret=pthread_create(&AEThreadID,NULL, &AEThread,this);
	if (ret != 0)
	{
		stringstream stream;
		stream << "Create pthread error!";
		LOG(ERROR, stream.str());

		exit(1);
	}
}

CodisClient::~CodisClient()
{
	//cout <<"~CodisClient()" << endl;
	if (m_ConnPool != NULL)
	{
		delete m_ConnPool;
		m_ConnPool = NULL;
	}
}

bool CodisClient::expire(string key, int seconds, int tt)
{
	Reply rep = RedisCommand(Command("EXPIRE")(key)(int2string(seconds)), tt);

	return rep.integer()==1;
}

bool CodisClient::exists(string key, int tt)
{
	Reply rep = RedisCommand(Command("EXISTS")(key), tt);

	return rep.integer()==1;

}

int CodisClient::del(string key, int tt)
{
	Reply rep = RedisCommand(Command("DEL")(key), tt);

	return rep.integer();
}

int CodisClient::del(vector<string>& keys, int tt)
{
	Command command("DEL");
	Reply rep;
	for (size_t i=0; i<keys.size(); i++)
	{
//		rep = RedisCommand(command(keys[i]));
		command(keys[i]);
	}

	rep = RedisCommand(command, tt);

	return rep.integer();
}

string CodisClient::type(string key, int tt)
{
	Reply rep = RedisCommand(Command("TYPE")(key), tt);

	return rep.str();
}

int CodisClient::setbit(string key, int index, bool value, int tt)
{
	Reply rep = RedisCommand(Command("SETBIT")(key)(int2string(index))(int2string(value)), tt);

	return rep.integer();
}

int CodisClient::getbit(string key, int index, int tt)
{
	Reply rep = RedisCommand(Command("GETBIT")(key)(int2string(index)), tt);

	return rep.integer();
}

int CodisClient::bitcount(string key, int tt)
{
	Reply rep = RedisCommand(Command("BITCOUNT")(key), tt);

	return rep.integer();
}

bool CodisClient::set(string key, string value, int tt)
{
	if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("SET")(key)(value), tt);

	return rep.str() == "OK";
}

bool CodisClient::setnx(string key, string value, int tt)
{
        if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("SETNX")(key)(value), tt);

	return rep.integer() == 1;
}

bool CodisClient::setex(string key, string value, int seconds, int tt)
{
        if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("SETEX")(key)(int2string(seconds))(value), tt);

	return rep.str() == "OK";
}

string CodisClient::get(string key, int tt)
{
	Reply rep = RedisCommand(Command("GET")(key), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

string CodisClient::getset(string key, string value, int tt)
{
        if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("GETSET")(key)(value), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

vector<string> CodisClient::mget(vector<string>& keys, int tt)
{
	vector<string> values;

	Command comm("MGET");
	for (size_t i=0; i<keys.size(); i++)
	{
		comm(keys[i]);
	}


	Reply rep = RedisCommand(comm, tt);

	if (rep.error())
	{
		return values;
	}
	else
	{
		for (size_t i=0; i<rep.elements().size(); i++)
		{
			values.push_back(rep.elements()[i].str());
		}
	}

	return values;
}

bool CodisClient::mget2(vector<string>& keys, void (*callback)(KVMap& kvs))
{
	vector<string> command;
	command.push_back("MGET");
	for (size_t i = 0; i < keys.size(); ++i)
	{
		if (m_BID!="")
		{
			command.push_back(m_BID + "_" + keys[i]);
		}
		else
		{
			command.push_back(keys[i]);
		}
	}

	vector<const char*> argv;
	vector<size_t> arglen;
	for (size_t i = 0; i < command.size(); ++i)
	{
		argv.push_back(command[i].c_str());
		arglen.push_back(command[i].size());
	}

	redisAsyncContext* async_context = m_ConnPool->borrowItemAsync(m_Loop);
	if (async_context == NULL)
	{
		LOG(ERROR, "contetx is NULL!!");
		m_ConnPool->returnItemAsync(async_context); // FIXME: add returnItem

		return false;
	}

	MgetAsyncRequestContext *mget_async_rctxt = new MgetAsyncRequestContext();
	mget_async_rctxt->user_keylist_ = keys;
	mget_async_rctxt->callback = callback;
	mget_async_rctxt->async_context = &async_context;
	mget_async_rctxt->client = this;

	int rc = redisAsyncCommandArgv(async_context, &mget2Callback,
			mget_async_rctxt, //FIXME: callback, privdata
			argv.size(), &argv[0], &arglen[0]);

	if (rc == REDIS_ERR)
	{
		if (!m_ConnPool->ReconnectAsync(async_context, m_Loop))
		{
			LOG(ERROR, "reconnect faild, ");
			m_ConnPool->returnItemAsync(async_context); // FIXME: add returnItem

			return false;
		}

		rc = redisAsyncCommandArgv(async_context, &mget2Callback, mget_async_rctxt, argv.size(),
				&argv[0], &arglen[0]);
	}

	if (rc == REDIS_ERR)
	{
		stringstream stream;
		stream << "run command error(" << rc << "): " << async_context->errstr;
		LOG(ERROR, stream.str());
		m_ConnPool->returnItemAsync(async_context); // FIXME: add returnItem

		return false;
	}

	//在callback中会把async_context放回连接池
//	m_ConnPool->returnItemAsync(async_context);

	return true;

	/*
	map<RedisDB, vector<string>  > client_map;

	vector<string> innerKey;

	for(size_t i=0; i<keys.size(); i++)
	{
		if (ifBid && m_BID!="")
		{
			innerKey.push_back(m_BID + "_" + keys[i]);
		}
		else
		{
			innerKey.push_back(keys[i]);
		}
	}

	for (size_t i=0; i<innerKey.size(); i++)
	{
		RedisDB db = m_KetamaHasher->get(innerKey[i]);

		client_map[db].push_back(innerKey[i]);
	}

	MgetAsyncResultMerger3 *merger = new MgetAsyncResultMerger3;
	merger->counter_ = client_map.size();
	merger->status_.resize(client_map.size());
	merger->result_ = &kvs;

	vector<AsyncInfo> infos;
	infos.resize(client_map.size());
	map<RedisDB, vector<string>  >::iterator iter = client_map.begin();
	for (size_t i=0; iter!=client_map.end(); ++iter, ++i)
	{
		infos[i].db = iter->first;
		infos[i].keys = iter->second;
		infos[i].mget_async_rctxt.merger_ = NULL;
		infos[i].mget_async_rctxt.user_keylist_ = iter->second;
	}

	for (size_t i=0; i<infos.size(); ++i)
	{
		MgetAsyncRequestContext3 *context = new MgetAsyncRequestContext3;
		context->merger_ = merger;
		context->user_keylist_ = infos[i].keys;

		bool ret = infos[i].db.mget3(infos[i].keys, context, loop_);
		merger->status_[i] = ret;
	}

	//mget3为异步调用, new的资源将在callback中释放
	 */

//	return true;
}

bool CodisClient::mset(map<string, string>& keyvalues, int tt)
{
	Command comm("MSET");
	map<string, string>::iterator it = keyvalues.begin();
	for (; it!=keyvalues.end(); it++)
	{
		if (it->second.length()>1048576){
                	throw myex;
        	}
		comm(it->first)(it->second);
	}

	Reply rep = RedisCommand(comm, tt);

	if (rep.error())
	{
		return false;
	}
	else
	{
		return true;
	}
}

int CodisClient::incr(string key, int tt)
{
	Reply rep = RedisCommand(Command("INCR")(key), tt);

	return (rep.error()) ? 0 : rep.integer();
}

int CodisClient::decr(string key, int tt)
{
	Reply rep = RedisCommand(Command("DECR")(key), tt);

	return (rep.error()) ? 0 : rep.integer();
}

int CodisClient::incrby(string key, int incr, int tt)
{

	Reply rep = RedisCommand(Command("INCRBY")(key)(int2string(incr)), tt);

	return (rep.error()) ? 0 : rep.integer();
}

int CodisClient::decrby(string key, int incr, int tt)
{
	Reply rep = RedisCommand(Command("DECRBY")(key)(int2string(incr)), tt);

	return (rep.error()) ? 0 : rep.integer();
}

long CodisClient::append(string key, string value, int tt)
{
	if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("APPEND")(key)(value), tt);

	return rep.integer();
}

int CodisClient::lpush(string key, string value, int tt)
{
	if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("LPUSH")(key)(value), tt);

	return rep.integer();
}

int CodisClient::rpush(string key, string value, int tt)
{
	if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("RPUSH")(key)(value), tt);

	return rep.integer();
}

int CodisClient::lpush(string key, vector<string> values, int tt)
{
	Command comm("LPUSH");
	comm(key);
	for (size_t i=0; i<values.size(); i++)
	{
		if (values[i].length()>1048576){
        	        throw myex;
	        }
		comm(values[i]);
	}

	Reply rep = RedisCommand(comm, tt);

	return rep.integer();
}

int CodisClient::rpush(string key, vector<string> values, int tt)
{
	Command comm("RPUSH");
	comm(key);
	for (size_t i=0; i<values.size(); i++)
	{
		if (values[i].length()>1048576){
         	       throw myex;
        	}
		comm(values[i]);
	}

	Reply rep = RedisCommand(comm, tt);

	return rep.integer();
}


int CodisClient::llen(string key, int tt)
{
	Reply rep = RedisCommand(Command("LLEN")(key), tt);

	return rep.integer();
}

vector<string> CodisClient::lrange(string key, int start, int end, int tt)
{
	vector<string> values;
	Reply rep = RedisCommand(Command("LRANGE")(key)(int2string(start))(int2string(end)), tt);

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

bool CodisClient::ltrim(string key, int start, int end, int tt)
{
	Reply rep = RedisCommand(Command("LTRIM")(key)(int2string(start))(int2string(end)), tt);

	return rep.str()==string("OK");
}

bool CodisClient::lset(string key, int index, string value, int tt)
{
	if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("LSET")(key)(int2string(index))(value), tt);

	return rep.str()==string("OK");
}

bool CodisClient::lrem(string key, int count, string value, int tt)
{
	Reply rep = RedisCommand(Command("LREM")(key)(int2string(count))(value), tt);

	return rep.integer();
}

string CodisClient::lpop(string key, int tt)
{
	Reply rep = RedisCommand(Command("LPOP")(key), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

string CodisClient::rpop(string key, int tt)
{
	Reply rep = RedisCommand(Command("RPOP")(key), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

bool CodisClient::sadd(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("SADD")(key)(member), tt);

	return rep.integer()==1;
}

int CodisClient::sadd(string key, vector<string> members, int tt)
{
	Command comm("SADD");
	comm(key);
	for (size_t i=0; i<members.size(); i++)
	{
		comm(members[i]);
	}

	Reply rep = RedisCommand(comm, tt);

	return rep.integer();
}

bool CodisClient::srem(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("SREM")(key)(member), tt);

	return rep.integer()==1;
}

string CodisClient::spop(string key, int tt)
{
	Reply rep = RedisCommand(Command("SPOP")(key), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

string CodisClient::srandmember(string key, int tt)
{
	Reply rep = RedisCommand(Command("SRANDMEMBER")(key), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

int CodisClient::scard(string key, int tt)
{
	Reply rep = RedisCommand(Command("SCARD")(key), tt);

	return rep.integer();
}

bool CodisClient::sismember(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("SISMEMBER")(key)(member), tt);

	return rep.integer()==1;
}

vector<string> CodisClient::smembers(string key, int tt)
{
	vector<string> values;
	Reply rep = RedisCommand(Command("SMEMBERS")(key), tt);

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

bool CodisClient::zadd(string key, int score, string member, int tt)
{
	Reply rep = RedisCommand(Command("ZADD")(key)(int2string(score))(member), tt);

	return rep.integer()==1;
}

bool CodisClient::zrem(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("ZREM")(key)(member), tt);

	return rep.integer()==1;
}

int CodisClient::zincrby(string key, int incr, string member, int tt)
{
	Reply rep = RedisCommand(Command("ZINCRBY")(key)(int2string(incr))(member), tt);

	return string2int(rep.str());
}

int CodisClient::zrank(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("ZRANK")(key)(member), tt);

	return rep.integer();
}

int CodisClient::zrevrank(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("ZREVRANK")(key)(member), tt);

	return rep.integer();
}

vector<string> CodisClient::zrange(string key, int start, int end, string withscores, int tt)
{
	vector<string> values;
        Reply rep;
	if (withscores=="True")
	{
		rep = RedisCommand(Command("ZRANGE")(key)(int2string(start))(int2string(end))("withscores"), tt);
	}
	else
	{
		rep = RedisCommand(Command("ZRANGE")(key)(int2string(start))(int2string(end)), tt);
	}

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

vector<string> CodisClient::zrevrange(string key, int start, int end, string withscores, int tt)
{
	vector<string> values;
	Reply rep;
        if (withscores=="True")
        {
		rep = RedisCommand(Command("ZREVRANGE")(key)(int2string(start))(int2string(end))("withscores"), tt);
	}
	else
	{
		rep = RedisCommand(Command("ZREVRANGE")(key)(int2string(start))(int2string(end)), tt);
	}

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

vector<string> CodisClient::zrangebyscore(string key, string min, string max, string withscores, int tt)
{
	vector<string> values;
	Reply rep;
        if (withscores=="True")
        {
		rep = RedisCommand(Command("ZRANGEBYSCORE")(key)(min)(max)("withscores"), tt);
	}
	else
	{
		rep = RedisCommand(Command("ZRANGEBYSCORE")(key)(min)(max), tt);
	}

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

vector<string> CodisClient::zrevrangebyscore(string key, string min, string max, string withscores, int tt)
{
	vector<string> values;
	Reply rep;
        if (withscores=="True")
        {
		rep = RedisCommand(Command("ZREVRANGEBYSCORE")(key)(min)(max)("withscores"), tt);
	}
	else
	{
		rep = RedisCommand(Command("ZREVRANGEBYSCORE")(key)(min)(max), tt);
	}

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

int CodisClient::zcount(string key, int min, int max, int tt)
{
	Reply rep = RedisCommand(Command("ZCOUNT")(key)(int2string(min))(int2string(max)), tt);

	return rep.integer();
}

int CodisClient::zcard(string key, int tt)
{
	Reply rep = RedisCommand(Command("ZCARD")(key), tt);

	return rep.integer();
}

int CodisClient::zscore(string key, string member, int tt)
{
	Reply rep = RedisCommand(Command("ZSCORE")(key)(member), tt);

	return string2int(rep.str());
}

int CodisClient::zremrangebyrank(string key, int min, int max, int tt)
{
	Reply rep = RedisCommand(Command("ZREMRANGEBYRANK")(key)(int2string(min))(int2string(max)), tt);

	return rep.integer();
}

int CodisClient::zremrangebyscore(string key, int min, int max, int tt)
{
	Reply rep = RedisCommand(Command("ZREMRANGEBYSCORE")(key)(int2string(min))(int2string(max)), tt);

	return rep.integer();
}

bool CodisClient::hset(string key, string field, string value, int tt)
{
	if (value.length()>1048576){
                throw myex;
        }
	Reply rep = RedisCommand(Command("HSET")(key)(field)(value), tt);

	return rep.integer();
}

string CodisClient::hget(string key, string field, int tt)
{
	Reply rep = RedisCommand(Command("HGET")(key)(field), tt);

	if (rep.error())
	{
		return "";
	}
	else
	{
		return rep.str();
	}
}

vector<string> CodisClient::hmget(string key, vector<string>& fields, int tt)
{
	vector<string> values;
	Command command("HMGET");
	command(key);
	for (size_t i=0; i<fields.size(); i++)
	{
		command(fields[i]);
	}

	Reply rep = RedisCommand(command, tt);

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

bool CodisClient::hmset(string key, vector<string>& fields, vector<string>& values, int tt)
{
	if (fields.size() != values.size()) return false;
	if (fields.empty()) return false;

	Command command("HMSET");
	command(key);

	for (size_t i=0; i<fields.size(); i++)
	{
		if (values[i].length()>1048576){
                	throw myex;
        	}
		command(fields[i])(values[i]);
	}

	Reply rep = RedisCommand(command, tt);

	return rep.str()==string("OK");
}

int CodisClient::hincrby(string key, string field, int incr, int tt)
{
	Reply rep = RedisCommand(Command("HINCRBY")(key)(field)(int2string(incr)), tt);

	return rep.integer();
}

bool CodisClient::hexists(string key, string field, int tt)
{
	Reply rep = RedisCommand(Command("HEXISTS")(key)(field), tt);

	return rep.integer()==1;
}

bool CodisClient::hdel(string key, string field, int tt)
{
	Reply rep = RedisCommand(Command("HDEL")(key)(field), tt);

	return rep.integer()==1;
}

int CodisClient::hlen(string key, int tt)
{
	Reply rep = RedisCommand(Command("HLEN")(key), tt);

	return rep.integer();
}

vector<string> CodisClient::hkeys(string key, int tt)
{
	vector<string> keys;
	Reply rep = RedisCommand(Command("HKEYS")(key), tt);

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		keys.push_back(rep.elements()[i].str());
	}

	return keys;
}

vector<string> CodisClient::hvals(string key, int tt)
{
	vector<string> values;
	Reply rep = RedisCommand(Command("HVALS")(key), tt);

	for (size_t i=0; i<rep.elements().size(); i++)
	{
		values.push_back(rep.elements()[i].str());
	}

	return values;
}

bool CodisClient::hgetall(string key, vector<string>& fields, vector<string>& values, int tt)
{
	Reply rep = RedisCommand(Command("HGETALL")(key), tt);

	if (rep.error()) return false;

	for (size_t i=1; i<rep.elements().size(); i+=2)
	{
		fields.push_back(rep.elements()[i-1].str());
		values.push_back(rep.elements()[i].str());
	}

	return true;
}

Reply CodisClient::RedisCommand(const vector<string>& command, int tt)
{
	if (command.size() < 2)
	{
		Reply reply;
		reply.SetErrorMessage("Command length should gt 2");
		return reply;
	}

	vector<string> innerCommand;

	for(size_t i=0; i<command.size(); i++)
	{
		if ((command[0] == "MGET") || (command[0] == "mget"))
		{
			if (m_BID!="" && i>0)
			{
				innerCommand.push_back(m_BID + "_" + command[i]);
			}
			else
			{
				innerCommand.push_back(command[i]);
			}
		}
		else if ((command[0] == "MSET") || (command[0] == "mset"))
		{
			if ((m_BID!="") && (i%2==1))
			{
				innerCommand.push_back(m_BID + "_" + command[i]);
			}
			else
			{
				innerCommand.push_back(command[i]);
			}
		}
		else if ((command[0] == "DEL") || (command[0] == "del"))
		{
			if ((m_BID!="") && (i>0))
			{
				innerCommand.push_back(m_BID + "_" + command[i]);
			}
			else
			{
				innerCommand.push_back(command[i]);
			}
		}
		else if (i==1 && m_BID!="")
		{
			innerCommand.push_back(m_BID + "_" + command[i]);
		}
		else
		{
			innerCommand.push_back(command[i]);
		}
	}

	vector<const char*> argv;
	vector<size_t> arglen;
	for (size_t i=0; i<innerCommand.size(); ++i)
	{
	    argv.push_back(innerCommand[i].c_str());
	    arglen.push_back(innerCommand[i].size());
	}
	if (!m_ConnPool)
	{
		LOG(ERROR, "connpool is null!");
		Reply reply;
		reply.SetErrorMessage("Can not fetch client from pool!!!");
		return reply;
	}
	//----
	redisContext* redis = m_ConnPool->borrowItem();
	if (!redis)
	{
	    LOG(ERROR, "contetx is null!");
	    Reply reply;
	    reply.SetErrorMessage("context is NULL !!");
	    return reply;
	}

	redisReply *reply;
	int ts = tt/1000;
	int tm = tt%1000;
	struct timeval tv = {ts, tm*1000};
        if (redisSetTimeout(redis,tv) != REDIS_OK){
	    redisFree(redis);
            redis = NULL;
            redis = m_ConnPool->create();
            if (redis == NULL)
            {
                LOG(ERROR, "reconnect faild!");
                Reply reply;
                reply.SetErrorMessage("reconnect faild!");
                return reply;
            }
	    redisSetTimeout(redis,tv);
	};
	reply = (redisReply*)redisCommandArgv(redis, argv.size(), &argv[0], &arglen[0]);
	// 服务端会主动关闭掉不活跃的连接，这里处理重练并重新发送命令
	//if (!reply)
	//{
	//    redisFree(redis);
	//    redis = NULL;
	//    redis = m_ConnPool->create();
	//    if (redis == NULL)
	//    {
	//    	LOG(ERROR, "reconnect faild!");
	//    	Reply reply;
	//    	reply.SetErrorMessage("reconnect faild!");
	//    	return reply;
	//    }
        //    assert(redisSetTimeout(redis,tv) == REDIS_OK);
	//    reply = (redisReply*)redisCommandArgv(redis, argv.size(), &argv[0], &arglen[0]);
	//}

	//// 重连后依然失败则返回
	m_ConnPool->returnItem(redis);
	if (!reply)
	{
		Reply reply;
		reply.SetErrorMessage("Do Command faild.");
		return reply;
	}

	// 如果命令发送成功则将连接放回到连接池
	//if (m_ConnPool != NULL) {
	//}
	//else {
//			cout << "m_ConnPool is NULL" << endl;
//	}
	Reply ret = Reply(reply);
	freeReplyObject(reply);
	return ret;
}

Reply CodisClient::RedisCommand(Command& command, int tt)
{
	return RedisCommand(command.args(), tt);
}

vector<Reply> CodisClient::RedisCommands(vector<Command>& commands)
{
	vector<Reply> replys;
        int tt = 0;
	for (size_t i=0; i<commands.size(); i++)
	{
		Reply reply = RedisCommand(commands[i], tt);
		replys.push_back(reply);
	}

	return replys;
}

void* CodisClient::AEThread(void *arg)
{
	CodisClient* ptr = reinterpret_cast<CodisClient*>(arg);

	aeMain(ptr->m_Loop);
}

void CodisClient::mget2Callback(redisAsyncContext *c, void *r, void *privdata)
{
	// mget异步调用的回调函数
	// 合并结果数据到merger, 在最后一个mget回调处理完成后，通过cv通知
	redisReply *reply = (redisReply*) r;
	MgetAsyncRequestContext *rctxt = (MgetAsyncRequestContext*) privdata;

	KVMap kvmap;
	//  printf("result count: %d\n", reply->elements);
	for (size_t i = 0; i < reply->elements; ++i)
	{
		string value = std::string(reply->element[i]->str,
				reply->element[i]->len);
		string user_key = rctxt->user_keylist_[i];
		//    printf("callback, key:(%s), value(%s), counter(%d)\n", user_key.c_str(), value.c_str(), rctxt->merger_->counter_);
		kvmap.insert(make_pair(user_key, value));
	}

	(rctxt->callback)(kvmap);

	rctxt->client->returnAsync(*(rctxt->async_context));

	if (rctxt != NULL)
	{
		delete rctxt;
		rctxt = NULL;
	}
}

