#include "RedisClientPool.h"
#include "ScopedLock.h"
#include "Utils.h"
#include "Log.h"

namespace bfd
{
namespace codis
{

RedisClientPool::RedisClientPool(const std::string& address, int port,
		const string & password, bool active, bool async, size_t coreSize,
		size_t maxSize) :
	address_(address), port_(port), active_(active), async_(async), coreSize_(
			coreSize), maxSize_(maxSize), used_(0), password_(password),
			unUsedMutex_(PTHREAD_MUTEX_INITIALIZER), unUsedAsyncMutex_(PTHREAD_MUTEX_INITIALIZER)
{
}

RedisClientPool::~RedisClientPool()
{
	ScopedLock lock(unUsedMutex_);
	for (std::deque<redisContext*>::iterator it = unUsed_.begin(); it
			!= unUsed_.end(); ++it)
	{
		stringstream stream;
		stream << "begin closing ~~~ mapping; redisContext* = " << (void *) *it;
		LOG(INFO, stream.str());
		Destroy(*it);
	}
}

redisContext* RedisClientPool::borrowItem()
{
	redisContext* rt = NULL;
	{
		ScopedLock lock(unUsedMutex_);
		{
		    while (!unUsed_.empty())
		    {
			    rt = unUsed_.front();
			    unUsed_.pop_front();
			    if (rt->err == REDIS_OK)
			    {
				    return rt;
			    }
			    else
			    {
				    redisFree(rt);
				    continue;
			    }
		    }
		    rt = create();
		    if (rt != NULL)
	        {
		        used_++;
	        }
		}
	}
	return rt;
}
// async
redisAsyncContext* RedisClientPool::borrowItemAsync(aeEventLoop *loop)
{
	redisAsyncContext* rt = NULL;
	{
		ScopedLock lock(unUsedAsyncMutex_);
		while (!unUsedAsync_.empty())
		{
			rt = unUsedAsync_.front();
			unUsedAsync_.pop_front();
			if (rt->err == REDIS_OK)
			{
				return rt;
			}
			else
			{
				// FIXME: asyncContext no need to free?
				//        redisFree(rt);
				continue;
			}
		}
	}
	rt = createAsync(loop);
	if (rt != NULL)
	{
		ScopedLock lock(unUsedAsyncMutex_);
		usedAsync_++;
	}
	return rt;
}

void RedisClientPool::returnItem(redisContext* item)
{
	ScopedLock lock(unUsedMutex_);
	unUsed_.push_back(item);
	used_--;
}
// async
void RedisClientPool::returnItemAsync(redisAsyncContext* item)
{
	ScopedLock lock(unUsedAsyncMutex_);
	unUsedAsync_.push_back(item);
	usedAsync_--;
}

redisContext* RedisClientPool::create()
{
	try
	{
		redisContext * ret = redisConnect(address_.c_str(), port_);
		if (ret != NULL && ret->err)
		{
			stringstream stream;
			stream << "Error: " << ret->errstr;
			LOG(ERROR, stream.str());

			fprintf(stderr, "Error: %s\n", ret->errstr);
			redisFree(ret);
			// 建立错误直接返回NULL
			ret = NULL;
			return ret;
		}
		if (password_ != "")
		{
			redisCommand(ret, "AUTH %s", password_.c_str());
		}
		return ret;
	} catch (std::exception &e)
	{
		return NULL;
	}
}

// async disconnect callback
void redisAsyncDisconnectCallback(const redisAsyncContext *c, int status)
{
	if (status == REDIS_ERR)
	{
		stringstream stream;
		stream << "Async Disconnect error: " << c->err;
		LOG(ERROR, stream.str());
	}
}
// async
redisAsyncContext* RedisClientPool::createAsync(aeEventLoop *loop)
{
	try
	{
		redisAsyncContext * ret = redisAsyncConnect(address_.c_str(), port_);

		if (ret != NULL && ret->err)
		{
			stringstream stream;
			stream << "reconnect Error: " << ret->err;
			LOG(ERROR, stream.str());
			// FIXME: no need to free
			//      redisFree(ret);
			// 建立错误直接返回NULL
			ret = NULL;
			return ret;
		}
		// 设置连接断开的回调函数
		redisAsyncSetDisconnectCallback(ret, redisAsyncDisconnectCallback);
		// 在第一次创建链接的时候做Attach操作
		redisAeAttach(loop, ret);
		return ret;
	} catch (std::exception &e)
	{
		return NULL;
	}
}

bool RedisClientPool::Reconnect(redisContext* rc)
{
	Destroy(rc);
	try
	{
		rc = redisConnect(address_.c_str(), port_);

		stringstream stream;
		stream << "connect redisContext->err: " << rc->err;
		LOG(INFO, stream.str());

		if (rc != NULL && rc->err != 0)
		{
			stringstream stream;
			stream << "reconnect Error: " << rc->errstr;
			LOG(ERROR, stream.str());
			// 建立错误直接返回NULL
			redisFree(rc);
			return false;
		}
	} catch (std::exception &e)
	{
		stringstream stream;
		stream << "reconnect server faild!!";
		LOG(ERROR, stream.str());

		return false;
	}
	return true;
}

// async
bool RedisClientPool::ReconnectAsync(redisAsyncContext* rc, aeEventLoop *loop)
{
	DestroyAsync(rc);
	try
	{
		rc = redisAsyncConnect(address_.c_str(), port_);
		//fprintf(stderr, "connect redisContext->err: %d\n", rc->err);
		if (rc != NULL && rc->err != 0)
		{
			stringstream stream;
			stream << "reconnect Error: " << rc->errstr;
			LOG(ERROR, stream.str());
			// 建立错误直接返回NULL
			// FIXME: no need to free?
			//      redisFree(rc);
			return false;
		}
		redisAsyncSetDisconnectCallback(rc, redisAsyncDisconnectCallback);
		// 在重连成功后做Attach操作
		redisAeAttach(loop, rc);
	} catch (std::exception &e)
	{
		stringstream stream;
		stream << "reconnect server faild!!";
		LOG(ERROR, stream.str());

		return false;
	}
	return true;
}

void RedisClientPool::Destroy(redisContext* item)
{
	if (item)
	{
		stringstream stream;
		stream << "free redis context";
		LOG(INFO, stream.str());

		redisFree(item);
	}
}

// async
void RedisClientPool::DestroyAsync(redisAsyncContext* item)
{
	if (item)
	{
		stringstream stream;
		stream << "free redis async context";
		LOG(INFO, stream.str());

		redisAsyncDisconnect(item);
		//redisFree(item);
	}
}

std::string RedisClientPool::getId()
{
	return address_ + ":" + int2string(port_);
}

}
}