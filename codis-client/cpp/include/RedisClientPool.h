// RedisClientPool.h
// owned by: www.baifendian.com
// contact: yi.wu@baifendian.com,xu.yan@baifendian.com
// RedisClientPool: 对同一个redis实例的连接池
// 同时报错多个redisContext*和多个redisAsyncContext*，针对不同的使用特点使用同步或异步的方式创建连接。
// 连接在第一次调用时创建，之后会复用多个连接
// NOTICE: RedisClientPool不暴露给最终client端使用，只用于clinet内部处理连接池

#ifndef _TRIPOD_REDISCLIENTPOOL_H__
#define _TRIPOD_REDISCLIENTPOOL_H__

#include <deque>
#include <cstdlib>
#include <string>
#include "hiredis.h"
#include "hiredis_ae.h"
#include "async.h"
#include "Utils.h"

using namespace std;

namespace bfd
{
namespace codis
{

class RedisClientPool
{
 public:
  RedisClientPool(const std::string& address, int port,
                  const string& password="",
                  bool active = true,
                  bool async = false,
                  size_t coreSize = 1,
                  size_t maxSize = 100);
  ~RedisClientPool();
  // 获取一个连接
  redisContext* borrowItem();
  redisAsyncContext* borrowItemAsync(aeEventLoop *loop);
  // 将连接放回pool中
  void returnItem(redisContext* item);
  void returnItemAsync(redisAsyncContext* item);
  // 创建连接
  redisContext* create();
  redisAsyncContext* createAsync(aeEventLoop *loop);
  // 重连
  bool Reconnect(redisContext*);
  bool ReconnectAsync(redisAsyncContext* rac, aeEventLoop *loop);
  // 销毁连接
  void Destroy(redisContext* item);
  void DestroyAsync(redisAsyncContext* item);
  // 获取当前pool的ip端口
  std::string getId();
  bool active() {return active_;};

 private:
  std::string address_;
  int port_;
  bool active_;
  bool async_;

  size_t coreSize_;
  size_t maxSize_;
  // 同步连接池
  pthread_mutex_t  unUsedMutex_;
  std::deque<redisContext*> unUsed_;
  size_t used_;
  // 异步连接池
  pthread_mutex_t unUsedAsyncMutex_;
  std::deque<redisAsyncContext*> unUsedAsync_;
  size_t usedAsync_;

  string password_;
};

}
}

#endif



