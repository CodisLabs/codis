#!/usr/bin/env python
#encoding:utf-8

import time
import logging
import os
import json,sys,threading
import redis
from kazoo.client import KazooClient

logging.basicConfig(filename="./bfdcodis.log", level=logging.DEBUG, format='[%(asctime)s %(levelname)s %(process)d %(filename)s %(lineno)d] - %(message)s')
logger = logging.getLogger()
lock = threading.Lock()
BOOLEAN = "False"


#@ brief 我的异常类
class MyException(Exception):
    #@ param[in] 异常信息
    def __init__(self,mess):
        self.message=mess

class BfdCodis(object):
    def __init__(self, zkAddr, proxyPath, businessID):
        self.__zkAddr = zkAddr
        self.__proxyPath = proxyPath
        self.__businessID = businessID
        self.__zk = KazooClient(zkAddr)
        self.__connPoolIndex = -1
        self.__connPool = []
        self.__InitFromZK()

    def __InitFromZK(self):
        global BOOLEAN
        BOOLEAN="True"
        lock.acquire()
        try:
            self.__proxylist = []
            self.__connPool = []
            self.__zk.start()
            proxynamelist = self.__zk.get_children(self.__proxyPath,watch=self.__watcher)
            for proxy in proxynamelist:
                self.__zk.start()
                proxyinfo = self.__zk.get(self.__proxyPath+'/'+proxy,watch=self.__watcher)
                decoded = json.loads(proxyinfo[0])
                if decoded["state"] == "online":
                    self.__proxylist.append(decoded)
            for proxyinfo in self.__proxylist:
                proxyip = proxyinfo["addr"].split(':')[0]
                proxyport = proxyinfo["addr"].split(':')[1]
                conn = redis.Redis(host=proxyip, port=int(proxyport))
                self.__connPool.append(conn)
        except Exception, e:
            logger.error("InitConnPool error")
        BOOLEAN="False"
        lock.release()
        
    def __getProxy(self):
        #global BOOLEAN
        #if BOOLEAN=="True":
        lock.acquire()
        self.__connPoolIndex += 1
        if self.__connPoolIndex >= len(self.__connPool):
            self.__connPoolIndex = 0
        if len(self.__connPool) == 0:
            lock.release()
            return None
        else:
            conn = self.__connPool[self.__connPoolIndex]
            lock.release()
            return conn
        #else:
        #    self.__connPoolIndex += 1
        #    if self.__connPoolIndex >= len(self.__connPool):
        #        self.__connPoolIndex = 0
        #    if len(self.__connPool) == 0:
        #        return None;
        #    else:
        #        return self.__connPool[self.__connPoolIndex]

    def __watcher(self, event):
        logger.debug("watcher callback type:%s state:%s path:%s"%(event.type,event.state,event.path))
        if event.type == "SESSION" and event.state == "CONNECTING":  
            #self.__zk.stop()
            #self.__zk = KazooClient(self.__zkAddr)
            pass
        elif event.type == "SESSION" and event.state == "EXPIRED_SESSION":  
            self.__zk.stop()
            self.__zk = KazooClient(self.__zkAddr)
        elif event.type == "CREATED" and event.state == "CONNECTED":  
            self.__InitFromZK()
        elif event.type == "DELETED" and event.state == "CONNECTED":  
            self.__InitFromZK()
        elif  event.type == "CHANGED" and event.state == "CONNECTED":  
            self.__InitFromZK()
        elif  event.type == "CHILD" and event.state == "CONNECTED":  
            self.__InitFromZK()
        else:
            logger.error("zookeeper connection state changed but not implemented: event:%s state:%s path:%s"%(event.type, event.state, event.path))
     
    def __convertKey(self, key):
        return self.__businessID + '_'+key
    
    def delete(self, *keys):
        innerkeys=[]
        for key in keys:
            innerkeys.append(self.__convertKey(key))
            
        try:
            return self.__getProxy().delete(*innerkeys)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().delete(*innerkeys)

    def strlen(self, key):
        try:
            return self.__getProxy().strlen(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().strlen(self.__convertKey(key))

    def exists(self, key):
        try:
            return self.__getProxy().exists(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().exists(self.__convertKey(key))
            
            
    def type(self, key):
        try:
            return self.__getProxy().type(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().type(self.__convertKey(key))
            
    def expire(self, key, time):
        try:
            return self.__getProxy().expire(self.__convertKey(key), time)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().expire(self.__convertKey(key), time)
            
    def getset(self, key, value):
        if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().getset(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().getset(self.__convertKey(key), value)
    
    def set(self, key, value):
	if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().set(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().set(self.__convertKey(key), value)
      
    def setnx(self, key, value):
        if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().setnx(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().setnx(self.__convertKey(key), value)

    def setex(self, key, value, time):
	if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().setex(self.__convertKey(key), value, time)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().setex(self.__convertKey(key), value, time)
    
    def append(self, key, value):
        if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().append(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().append(self.__convertKey(key), value)
    
    def get(self, key):
        try:
            return self.__getProxy().get(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().get(self.__convertKey(key))
			
	def ttl(self, key):
        try:
            return self.__getProxy().ttl(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().ttl(self.__convertKey(key))
    
    def list_or_args(self, keys, args):
        # returns a single list combining keys and args
        try:
            i = iter(keys)
            # a string can be iterated, but indicates
            # keys wasn't passed as a list
            if isinstance(keys, basestring):
                keys = [keys]
        except TypeError:
            keys = [keys]
        if args:
            keys.extend(args)
        return keys

    def mget(self, keys, *args):
        """
        Returns a list of values ordered identically to ``keys``
        """
        keys = self.list_or_args(keys, args)
        innerkeys=[]
        for key in keys:
            innerkey=self.__businessID+'_'+key
            innerkeys.append(innerkey)
        try:
            return self.__getProxy().mget(*innerkeys)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().mget(*innerkeys)

    def mset(self, mapping):
        kvs = {}
        for key, value in mapping.iteritems():
            if len(value)>1048576:
                raise MyException('the value is too bigger than 1M')
            kvs[self.__businessID+'_'+key] = value
        try:
            return self.__getProxy().mset(kvs)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().mset(kvs)
    
    def msetnx(self, mapping):
        kvs = {}
        for key, value in mapping.iteritems():
            if len(value)>1048576:
                raise MyException('the value is too bigger than 1M')
            kvs[self.__businessID+'_'+key] = value
        try:
            return self.__getProxy().msetnx(kvs)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().msetnx(kvs)

    def decr(self, key, amount=1):
        try:
            return self.__getProxy().decr(self.__convertKey(key), amount)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().decr(self.__convertKey(key), amount)
    
    def incr(self, key, amount=1):
        try:
            return self.__getProxy().incr(self.__convertKey(key), amount)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().incr(self.__convertKey(key), amount)
    
    def llen(self, key):
        try:
            return self.__getProxy().llen(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().llen(self.__convertKey(key))
    
    def lpop(self, key):
        try:
            return self.__getProxy().lpop(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().lpop(self.__convertKey(key))
    
    def lpush(self, key, *value):
        try:
            return self.__getProxy().lpush(self.__convertKey(key), *value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().lpush(self.__convertKey(key), *value)
      
    def lrange(self, key, start, end):
        try:
            return self.__getProxy().lrange(self.__convertKey(key), start, end)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().lrange(self.__convertKey(key), start, end)
    
    def lrem(self, key, value, num=0):
        """
        Remove the first ``num`` occurrences of ``value`` from list ``name``

        If ``num`` is 0, then all occurrences will be removed
        """
        try:
            return self.__getProxy().lrem(self.__convertKey(key), value, num)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().lrem(self.__convertKey(key), value, num)
    
    def lset(self, key, index, value):
        if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().lset(self.__convertKey(key), index, value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().lset(self.__convertKey(key), index, value)
    
    def ltrim(self, key, start, end):
        try:
            return self.__getProxy().ltrim(self.__convertKey(key), start, end)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().ltrim(self.__convertKey(key), start, end)
    
    def rpop(self, key):
        try:
            return self.__getProxy().rpop(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().rpop(self.__convertKey(key))
    
    def rpush(self, key, *value):
        try:
            return self.__getProxy().rpush(self.__convertKey(key), *value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().rpush(self.__convertKey(key), *value)
    
    def sadd(self, key, *value):
        try:
            return self.__getProxy().sadd(self.__convertKey(key), *value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().sadd(self.__convertKey(key), *value)
    
    def scard(self, key):
        try:
            return self.__getProxy().scard(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().scard(self.__convertKey(key))
    
    def sismember(self, key, value):
        try:
            return self.__getProxy().sismember(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().sismember(self.__convertKey(key), value)
    
    def smembers(self, key):
        try:
            return self.__getProxy().smembers(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().smembers(self.__convertKey(key))
    
    def spop(self, key):
        try:
            return self.__getProxy().spop(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().spop(self.__convertKey(key))
    
    def srandmember(self, key):
        try:
            return self.__getProxy().srandmember(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().srandmember(self.__convertKey(key))
    
    def srem(self, key, value):
        try:
            return self.__getProxy().srem(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().srem(self.__convertKey(key), value)
    
    def zadd(self, key, *value, **pairs):
        try:
            return self.__getProxy().zadd(self.__convertKey(key), *value, **pairs)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zadd(self.__convertKey(key), *value, **pairs)
    
    def zcard(self, key):
        try:
            return self.__getProxy().zcard(self.__convertKey(key))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zcard(self.__convertKey(key))
    
    def zcount(self, key, min, max):
        try:
            return self.__getProxy().zcount(self.__convertKey(key), min, max)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zcount(self.__convertKey(key), min, max)
    
    def zincrby(self, key, value, amount=1):
        try:
            return self.__getProxy().zincrby(self.__convertKey(key), value, amount)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zincrby(self.__convertKey(key), value, amount)
    
    def zrange(self, key, start, end, desc=False, withscores=False):
        try:
            return self.__getProxy().zrange(self.__convertKey(key), start, end, desc, withscores)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zrange(self.__convertKey(key), start, end, desc, withscores)
    
    def zrangebyscore(self, key, min, max,
            start=None, num=None, withscores=False):
        try:
            return self.__getProxy().zrangebyscore(self.__convertKey(key), min, max, start, num, withscores)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zrangebyscore(self.__convertKey(key), min, max, start, num, withscores)
    
    def zrank(self, key, value):
        try:
            return self.__getProxy().zrank(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zrank(self.__convertKey(key), value)
    
    def zrem(self, key, value):
        try:
            return self.__getProxy().zrem(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zrem(self.__convertKey(key), value)
    
    def zremrangebyrank(self, key, min, max):
        try:
            return self.__getProxy().zremrangebyrank(self.__convertKey(key), min, max)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zremrangebyrank(self.__convertKey(key), min, max)
    
    def zremrangebyscore(self, key, min, max):
        try:
            return self.__getProxy().zremrangebyscore(self.__convertKey(key), min, max)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zremrangebyscore(self.__convertKey(key), min, max)
        
    def zrevrange(self, key, start, num, withscores=False):
        try:
            return self.__getProxy().zrevrange(self.__convertKey(key), start, num, withscores)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zrevrange(self.__convertKey(key), start, num, withscores)
    
    def zrevrangebyscore(self, key, max, min,
            start=None, num=None, withscores=False):
        try:
            return self.__getProxy().zrevrangebyscore(self.__convertKey(key), min, max, start, num, withscores)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zrevrangebyscore(self.__convertKey(key), min, max, start, num, withscores)
    
    def zscore(self, key, value):
        try:
            return self.__getProxy().zscore(self.__convertKey(key), value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().zscore(self.__convertKey(key), value)
    
    def hdel(self, name, *keys):
        try:
            return self.__getProxy().hdel(self.__convertKey(name), *keys)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hdel(self.__convertKey(name), *keys)
    
    def hexists(self, name, key):
        try:
            return self.__getProxy().hexists(self.__convertKey(name), key)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hexists(self.__convertKey(name), key)
    
    def hget(self, name, key):
        try:
            return self.__getProxy().hget(self.__convertKey(name), key)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hget(self.__convertKey(name), key)
    
    
    def hgetall(self, name):
        try:
            return self.__getProxy().hgetall(self.__convertKey(name))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hgetall(self.__convertKey(name))
    
    def hincrby(self, name, key, amount=1):
        try:
            return self.__getProxy().hincrby(self.__convertKey(name), key, amount)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hincrby(self.__convertKey(name), key, amount)
    
    def hkeys(self, name):
        try:
            return self.__getProxy().hkeys(self.__convertKey(name))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hkeys(self.__convertKey(name))
    
    def hlen(self, name):
        try:
            return self.__getProxy().hlen(self.__convertKey(name))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hlen(self.__convertKey(name))
    
    def hset(self, name, key, value):
        if len(value)>1048576:
            raise MyException('the value is too bigger than 1M')
        try:
            return self.__getProxy().hset(self.__convertKey(name), key, value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hset(self.__convertKey(name), key, value)
    
    def hmset(self, name, mapping):
        try:
            return self.__getProxy().hmset(self.__convertKey(name), mapping)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hmset(self.__convertKey(name), mapping)
    
    def hmget(self, name, *keys):
        try:
            return self.__getProxy().hmget(self.__convertKey(name), *keys)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hmget(self.__convertKey(name), *keys)
    
    def hvals(self, name):
        try:
            return self.__getProxy().hvals(self.__convertKey(name))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().hvals(self.__convertKey(name))

    def pfadd(self, key, *value):
        try:
            return self.__getProxy().pfadd(self.__convertKey(key), *value)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().pfadd(self.__convertKey(key), *value)

    def pfcount(self, name):
        try:
            return self.__getProxy().pfcount(self.__convertKey(name))
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().pfcount(self.__convertKey(name))

    def pfmerge(self, keys, *args):
        keys = self.list_or_args(keys, args)
        innerkeys=[]
        for key in keys:
            innerkey=self.__businessID+'_'+key
            innerkeys.append(innerkey)
        try:
            return self.__getProxy().pfmerge(*innerkeys)
        except redis.exceptions.ConnectionError, e:
            return self.__getProxy().pfmerge(*innerkeys)
    
    def getResource(self):
        return self.__getProxy()

    def close(self):
        try:
            self.__zk.stop()
            self.__connPool = []
        except Exception, e:
            pass
        finally:
            logger.info("release info")
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
    
      
