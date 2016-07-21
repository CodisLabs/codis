package com.wandoulabs.jodis;

import java.util.ArrayList;
import java.util.AbstractMap;
import java.util.List;
import java.util.Map;
import java.util.Map.Entry;
import java.util.Set;
import java.lang.Exception;

import redis.clients.jedis.*;

class ValueException extends Exception  
{  
    public ValueException(String msg)  
    {  
        super(msg);  
    }  
}   
	

public class BfdJodis {
	
	private JedisResourcePool jedisPool = null;
	private String businessID = "";
	
    /**
     * 
     * @param zkAddr
     *            ZooKeeper connect string. e.g., "zk1:2181"
     * @param zkSessionTimeoutMs
     *            ZooKeeper session timeout in ms
     * @param zkPath
     *            the codis proxy dir on ZooKeeper. e.g.,
     *            "/zk/codis/db_xxx/proxy"
     * @param poolConfig
     *            same as JedisPool
     * @param timeout
     *            timeout of JedisPool
     * @param businessID
     *            your business ID
     */
	public BfdJodis(String zkAddr, int zkSessionTimeoutMs, String zkPath,
            JedisPoolConfig poolConfig, int timeout,String businessID) {
		
		this.jedisPool = new RoundRobinJedisPool(zkAddr, zkSessionTimeoutMs, zkPath, poolConfig,timeout);
		this.businessID=businessID;
	}
	
    /**
     * 
     * @param zkAddr
     *            ZooKeeper connect string. e.g., "zk1:2181"
     * @param zkSessionTimeoutMs
     *            ZooKeeper session timeout in ms
     * @param zkPath
     *            the codis proxy dir on ZooKeeper. e.g.,
     *            "/zk/codis/db_xxx/proxy"
     * @param poolConfig
     *            same as JedisPool
     * @param businessID
     *            your business ID
     */
	public BfdJodis(String zkAddr, int zkSessionTimeoutMs, String zkPath,
            JedisPoolConfig poolConfig, String businessID) {
		
		this.jedisPool = new RoundRobinJedisPool(zkAddr, zkSessionTimeoutMs, zkPath, poolConfig);
		this.businessID=businessID;
	}
	
	private String  AddBid(String key){
		return this.businessID+'_'+key;
	}
	
	private String[] AddBids(String... keys){
		String[] ret=new String[keys.length];
		for (int i=0;i<keys.length;i++) {
			ret[i]=this.businessID+'_'+keys[i];
		}
		return ret;
	}
	
	private String[] AddBidsDiv(String... keys) throws ValueException{
		String[] ret=new String[keys.length];
		for (int i=0;i<keys.length;i++) {
			if (0==(i %2))
				ret[i]=this.businessID+'_'+keys[i];
			else
				if (keys[i].length()>1048576){
                    			throw new ValueException("the value is too bigger than 1M");
                		}	
				ret[i]=keys[i];
		}
		return ret;
	}
	
	public String set(final String key, String value) throws ValueException {
		if (value.length()>1048576){
		    throw new ValueException("the value is too bigger than 1M");
		}
                try
                {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.set(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.set(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
	}
	
	public String setBytes(final String key, final byte[] value) {
	    try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.set(AddBid(key).getBytes(),value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.set(AddBid(key).getBytes(),value);
		    jediscon.close();
		    return ret;
		}
	}
	
	public byte[] getBytes(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    byte[] ret =jediscon.get(AddBid(key).getBytes());
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    byte[] ret =jediscon.get(AddBid(key).getBytes());
		    jediscon.close();
		    return ret;
		}
	}
	
	public String get(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.get(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.get(AddBid(key));
		    jediscon.close();
		    return ret;
		}
    }
	public List<String> mget(final String... keys) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.mget(AddBids(keys));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.mget(AddBids(keys));
		    jediscon.close();
		    return ret;
		}
	}
	public String mset(final String... keysvalues) throws ValueException {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.mset(AddBidsDiv(keysvalues));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.mset(AddBidsDiv(keysvalues));
		    jediscon.close();
		    return ret;
		}
	}

	
	public Boolean exists(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Boolean ret =jediscon.exists(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Boolean ret =jediscon.exists(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long del(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.del(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.del(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long del(final String... keys) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.del(AddBids(keys));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.del(AddBids(keys));
		    jediscon.close();
		    return ret;
		}
	}
	public String type(final String key) { 
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.type(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.type(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long expire(final String key, final int seconds) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.expire(AddBid(key),seconds);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.expire(AddBid(key),seconds);
		    jediscon.close();
		    return ret;
		}
	}
	public String setex(final String key, final int seconds, final String value) {
         try
         {
             Jedis jediscon= jedisPool.getResource();
             String ret =jediscon.setex(AddBid(key),seconds,value);
             jediscon.close();
             return ret;
         }
         catch (Exception e)
         {
             Jedis jediscon= jedisPool.getResource();
             String ret =jediscon.setex(AddBid(key),seconds,value);
             jediscon.close();
             return ret;
         }
	}
	public Long setnx(final String key, final String value) throws ValueException {
		if (value.length()>1048576){
                    throw new ValueException("the value is too bigger than 1M");
                }
		try
        	{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.setnx(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.setnx(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
	}
	public String getSet(final String key, final String value) throws ValueException { 
		if (value.length()>1048576){
                    throw new ValueException("the value is too bigger than 1M");
                }
		try
        	{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.getSet(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.getSet(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
	}
	public Long decr(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.decr(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.decr(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long incr(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.incr(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.incr(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long incrBy(final String key, final long integer) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.incrBy(AddBid(key),integer);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.incrBy(AddBid(key),integer);
		    jediscon.close();
		    return ret;
		}
	}
	public Long decrBy(final String key, final long integer) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.decrBy(AddBid(key),integer);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.decrBy(AddBid(key),integer);
		    jediscon.close();
		    return ret;
		}
	}
	public Long append(final String key, final String value) throws ValueException {
		if (value.length()>1048576){
                    throw new ValueException("the value is too bigger than 1M");
                }
		try
        	{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.append(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.append(AddBid(key),value);
		    jediscon.close();
		    return ret;
		}
	}
	public Long llen(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.llen(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.llen(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public List<String> lrange(final String key, final long start, final long end) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.lrange(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.lrange(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
	}
	public String ltrim(final String key, final long start, final long end) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.ltrim(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.ltrim(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
	}
	public String lset(final String key, final long index, final String value) throws ValueException {
		if (value.length()>1048576){
                    throw new ValueException("the value is too bigger than 1M");
                }
		try
        	{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.lset(AddBid(key),index,value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.lset(AddBid(key),index,value);
		    jediscon.close();
		    return ret;
		}
	}
	public Long lrem(final String key, final long count, final String value) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.lrem(AddBid(key),count,value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.lrem(AddBid(key),count,value);
		    jediscon.close();
		    return ret;
		}
	}
	public String lpop(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.lpop(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.lpop(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long lpush(final String key, final String... strings) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.lpush(AddBid(key),strings);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.lpush(AddBid(key),strings);
		    jediscon.close();
		    return ret;
		}
	}
	public Long rpush(final String key, final String... strings) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.rpush(AddBid(key),strings);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.rpush(AddBid(key),strings);
		    jediscon.close();
		    return ret;
		}
	}
	public String rpop(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.rpop(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.rpop(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public String spop(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.spop(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.spop(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long scard(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.scard(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.scard(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Boolean sismember(final String key, final String member) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Boolean ret =jediscon.sismember(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Boolean ret =jediscon.sismember(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
	}
	public Set<String> smembers(final String key) {
	    try
        {
		    Jedis jediscon= jedisPool.getResource();
	        Set<String> ret =jediscon.smembers(AddBid(key));
	        jediscon.close();
	        return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
	        Set<String> ret =jediscon.smembers(AddBid(key));
	        jediscon.close();
	        return ret;
		}
    }
	public Long sadd(final String key, final String... members) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.sadd(AddBid(key),members);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.sadd(AddBid(key),members);
		    jediscon.close();
		    return ret;
		}
	}
	public Long srem(final String key, final String... members) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.srem(AddBid(key),members);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.srem(AddBid(key),members);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zadd(final String key, final double score, final String member) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zadd(AddBid(key),score,member);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zadd(AddBid(key),score,member);
		    jediscon.close();
		    return ret;
		}
	}
	public Double zincrby(final String key, final double score, final String member) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Double ret =jediscon.zincrby(AddBid(key),score,member);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Double ret =jediscon.zincrby(AddBid(key),score,member);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zrem(final String key, final String... members) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zrem(AddBid(key),members);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zrem(AddBid(key),members);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zrank(final String key, final String member) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zrank(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zrank(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zrevrank(final String key, final String member) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zrevrank(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zrevrank(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
	}
	public Set<String> zrange(final String key, final long start, final long end) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.zrange(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.zrange(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
	}
	public Set<String> zrevrange(final String key, final long start, final long end) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.zrevrange(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.zrevrange(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
	}
	public Set<String> zrangeByScore(final String key, final double min, final double max) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.zrangeByScore(AddBid(key),min,max);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.zrangeByScore(AddBid(key),min,max);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zcount(final String key, final double min, final double max) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zcount(AddBid(key),min,max);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zcount(AddBid(key),min,max);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zcard(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zcard(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zcard(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Double zscore(final String key, final String member) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Double ret =jediscon.zscore(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Double ret =jediscon.zscore(AddBid(key),member);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zremrangeByRank(final String key, final long start, final long end) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zremrangeByRank(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zremrangeByRank(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
	}
	public Long zremrangeByScore(final String key, final double start, final double end) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zremrangeByScore(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.zremrangeByScore(AddBid(key),start,end);
		    jediscon.close();
		    return ret;
		}
	}
	public Long hset(final String key, final String field, final String value) throws ValueException {
		if (value.length()>1048576){
                    throw new ValueException("the value is too bigger than 1M");
                }
		try
        	{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hset(AddBid(key),field,value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hset(AddBid(key),field,value);
		    jediscon.close();
		    return ret;
		}
	}
	public String hget(final String key, final String field) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.hget(AddBid(key),field);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.hget(AddBid(key),field);
		    jediscon.close();
		    return ret;
		}
	}
	public List<String> hmget(final String key, final String... fields) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.hmget(AddBid(key),fields);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.hmget(AddBid(key),fields);
		    jediscon.close();
		    return ret;
		}
	}
	public String hmset(final String key, final Map<String, String> hash) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.hmset(AddBid(key),hash);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    String ret =jediscon.hmset(AddBid(key),hash);
		    jediscon.close();
		    return ret;
		}
	}
	public Long hincrBy(final String key, final String field, final long value) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hincrBy(AddBid(key),field,value);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hincrBy(AddBid(key),field,value);
		    jediscon.close();
		    return ret;
		}
	}
	public Boolean hexists(final String key, final String field) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Boolean ret =jediscon.hexists(AddBid(key),field);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Boolean ret =jediscon.hexists(AddBid(key),field);
		    jediscon.close();
		    return ret;
		}
	}
	public Long hdel(final String key, final String... fields) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hdel(AddBid(key),fields);
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hdel(AddBid(key),fields);
		    jediscon.close();
		    return ret;
		}
	}
	public Long hlen(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hlen(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Long ret =jediscon.hlen(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Set<String> hkeys(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.hkeys(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Set<String> ret =jediscon.hkeys(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public List<String> hvals(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.hvals(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    List<String> ret =jediscon.hvals(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Map<String, String> hgetAll(final String key) {
		try
        {
		    Jedis jediscon= jedisPool.getResource();
		    Map<String, String> ret =jediscon.hgetAll(AddBid(key));
		    jediscon.close();
		    return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
		    Map<String, String> ret =jediscon.hgetAll(AddBid(key));
		    jediscon.close();
		    return ret;
		}
	}
	public Long pfadd(final String key, final String... strings) {
        try
        {
		    Jedis jediscon= jedisPool.getResource();
            Long ret =jediscon.pfadd(AddBid(key),strings);
            jediscon.close();
            return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
            Long ret =jediscon.pfadd(AddBid(key),strings);
            jediscon.close();
            return ret;
		}
    }
    public Long pfcount(final String key) {
        try
        {
		    Jedis jediscon= jedisPool.getResource();
            Long ret =jediscon.pfcount(AddBid(key));
            jediscon.close();
            return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
            Long ret =jediscon.pfcount(AddBid(key));
            jediscon.close();
            return ret;
		}
    }
    public String pfmerge(final String key, final String... strings) {
        try
        {
		    Jedis jediscon= jedisPool.getResource();
            String ret =jediscon.pfmerge(AddBid(key),AddBids(strings));
            jediscon.close();
            return ret;
		}
		catch (Exception e)
		{
		    Jedis jediscon= jedisPool.getResource();
            String ret =jediscon.pfmerge(AddBid(key),AddBids(strings));
            jediscon.close();
            return ret;
		}
    }
}
