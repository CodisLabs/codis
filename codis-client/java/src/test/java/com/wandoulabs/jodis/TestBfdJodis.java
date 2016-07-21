import static org.junit.Assert.assertEquals;
import com.wandoulabs.jodis.BfdJodis;
import java.util.List;
import redis.clients.jedis.JedisPoolConfig;
import java.lang.Exception;

public class TestBfdJodis {

	public static void main(String[] args) {
		// TODO Auto-generated method stub
		BfdJodis bfdjodis = new BfdJodis("192.168.161.22:2181", 1000, "/zk/codis/db_test23/proxy",
				new JedisPoolConfig(), 1000, "bfd");
	try{
        String str = bfdjodis.set("k1","v1");
    	   	
    	System.out.println("value="+str);
        
        assertEquals("v1",bfdjodis.get("k1"));
    	
    	assertEquals(true,bfdjodis.exists("k1"));
   	
    	bfdjodis.set("k2","v2");
    	bfdjodis.set("k3","v3");
    	bfdjodis.set("k4","v5");
    	bfdjodis.set("k5","v5");
    
    	bfdjodis.del("k2");
    	
    	assertEquals(false,bfdjodis.exists("k2"));
    	
    	bfdjodis.del("k3","k4");
		bfdjodis.setex("k31",100,"k41");
    	
    	assertEquals(false,bfdjodis.exists("k3"));
    	assertEquals(false,bfdjodis.exists("k4"));
    	
    	assertEquals(bfdjodis.type("k5"),bfdjodis.type("k5"));
    	
    	
    	bfdjodis.expire("k5",0);
    	
    	assertEquals(false,bfdjodis.exists("k5"));
    	
    	bfdjodis.del("k6");
    	bfdjodis.setnx("k6","v6");
    	
    	assertEquals("v6",bfdjodis.get("k6"));
    	
    	bfdjodis.getSet("k6","v7");
    	
    	assertEquals("v7",bfdjodis.get("k6"));
    	
    	bfdjodis.mset("k8","v8","k9","v9");
    	
    	assertEquals("v8",bfdjodis.get("k8"));
    	assertEquals("v9",bfdjodis.get("k9"));
    	
    	List<String> ret=bfdjodis.mget("k8","k9");
    	assertEquals("v8",ret.get(0));
    	
    	bfdjodis.del("k12");
    	assertEquals(1,bfdjodis.incr("k12").intValue());
    	
    	
    	assertEquals(0,bfdjodis.decr("k12").intValue());
    	
    	
    	assertEquals(10,bfdjodis.incrBy("k12",10).intValue());
    	
    	
    	assertEquals(5,bfdjodis.decrBy("k12",5).intValue());
    	
    	
    	bfdjodis.append("k1","1");
    	
    	assertEquals("v11",bfdjodis.get("k1"));
    	//list
    	
    	bfdjodis.del("list");
    	assertEquals(2,bfdjodis.lpush("list","a","b").intValue());
    	
    	
    	assertEquals(2,bfdjodis.llen("list").intValue());
    	
    	//set
    	bfdjodis.del("set");
    	assertEquals(1,bfdjodis.sadd("set","a").intValue());
    	
    	
    	assertEquals(true,bfdjodis.sismember("set","a"));
    
    	//zset
    	
    	bfdjodis.zadd("zset",10,"a");
    	
    	assertEquals(10,bfdjodis.zscore("zset","a").intValue());
    	
    	assertEquals(0,bfdjodis.zrank("zset","a").intValue());
    	
    	//hash
    	
    	bfdjodis.hset("hash","a","1");
    	
    	assertEquals("1",bfdjodis.hget("hash","a"));
    	
    	assertEquals(1,bfdjodis.hlen("hash").intValue());
    			
	}catch(Exception yx)  
        {  
            System.out.println(yx.getMessage());  
            yx.printStackTrace();  
        } 
	}

}
