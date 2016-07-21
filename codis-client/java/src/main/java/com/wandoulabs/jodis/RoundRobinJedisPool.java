/**
 * @(#)RoundRobinJedisPool.java, 2014-11-30.
 * 
 * Copyright (c) 2014 Wandoujia Inc.
 * 
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 * 
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
 * LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
 * OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
 * WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
package com.wandoulabs.jodis;

import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

import org.apache.curator.framework.CuratorFramework;
import org.apache.curator.framework.CuratorFrameworkFactory;
import org.apache.curator.framework.imps.CuratorFrameworkState;
import org.apache.curator.framework.recipes.cache.ChildData;
import org.apache.curator.framework.recipes.cache.PathChildrenCache;
import org.apache.curator.framework.recipes.cache.PathChildrenCache.StartMode;
import org.apache.curator.framework.recipes.cache.PathChildrenCacheEvent;
import org.apache.curator.framework.recipes.cache.PathChildrenCacheListener;
import org.apache.log4j.Logger;

import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPool;
import redis.clients.jedis.JedisPoolConfig;
import redis.clients.jedis.exceptions.JedisException;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableSet;
import com.google.common.collect.Maps;
import com.google.common.collect.Sets;
import com.google.common.io.Closeables;

/**
 * A round robin connection pool for connecting multiple codis proxies based on
 * Jedis and Curator.
 * 
 * @author Apache9
 * @see https://github.com/xetorthio/jedis
 * @see http://curator.apache.org/
 */
public class RoundRobinJedisPool implements JedisResourcePool {

    private static final Logger LOG = Logger.getLogger(RoundRobinJedisPool.class);

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static final String JSON_NAME_CODIS_PROXY_ADDR = "addr";

    private static final String JSON_NAME_CODIS_PROXY_STATE = "state";

    private static final String CODIS_PROXY_STATE_ONLINE = "online";

    private static final int CURATOR_RETRY_BASE_SLEEP_MS = 100;

    private static final int CURATOR_RETRY_MAX_SLEEP_MS = 30 * 1000;

    private static final int JEDIS_POOL_TIMEOUT_UNSET = -1;

    private static final ImmutableSet<PathChildrenCacheEvent.Type> RESET_TYPES = Sets
            .immutableEnumSet(PathChildrenCacheEvent.Type.CHILD_ADDED,
                    PathChildrenCacheEvent.Type.CHILD_UPDATED,
                    PathChildrenCacheEvent.Type.CHILD_REMOVED);

    private final CuratorFramework curatorClient;

    private final boolean closeCurator;

    private final PathChildrenCache watcher;

    private static final class PooledObject {
        public final String addr;

        public final JedisPool pool;

        public PooledObject(String addr, JedisPool pool) {
            this.addr = addr;
            this.pool = pool;
        }

    }

    private volatile ImmutableList<PooledObject> pools = ImmutableList.of();

    private final AtomicInteger nextIdx = new AtomicInteger(-1);

    private final JedisPoolConfig poolConfig;

    private final int timeout;

    /**
     * Create a RoundRobinJedisPool with default timeout.
     * <p>
     * We create a CuratorFramework with infinite retry number. If you do not
     * like the behavior, use the other constructor that allow you pass a
     * CuratorFramework created by yourself.
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
     * @see #RoundRobinJedisPool(String, int, String, JedisPoolConfig, int)
     */
    public RoundRobinJedisPool(String zkAddr, int zkSessionTimeoutMs, String zkPath,
            JedisPoolConfig poolConfig) {
        this(zkAddr, zkSessionTimeoutMs, zkPath, poolConfig, JEDIS_POOL_TIMEOUT_UNSET);
    }

    /**
     * Create a RoundRobinJedisPool.
     * <p>
     * We create a CuratorFramework with infinite retry number. If you do not
     * like the behavior, use the other constructor that allow you pass a
     * CuratorFramework created by yourself.
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
     * @see #RoundRobinJedisPool(CuratorFramework, boolean, String,
     *      JedisPoolConfig, int)
     */
    public RoundRobinJedisPool(String zkAddr, int zkSessionTimeoutMs, String zkPath,
            JedisPoolConfig poolConfig, int timeout) {
        this(CuratorFrameworkFactory
                .builder()
                .connectString(zkAddr)
                .sessionTimeoutMs(zkSessionTimeoutMs)
                .retryPolicy(
                        new BoundedExponentialBackoffRetryUntilElapsed(CURATOR_RETRY_BASE_SLEEP_MS,
                                CURATOR_RETRY_MAX_SLEEP_MS, -1L)).build(), true, zkPath,
                poolConfig, timeout);
    }

    /**
     * Create a RoundRobinJedisPool with default timeout.
     * 
     * @param curatorClient
     *            We will start it if it has not started yet.
     * @param closeCurator
     *            Whether to close the curatorClient passed in when close.
     * @param zkPath
     *            the codis proxy dir on ZooKeeper. e.g.
     *            "/zk/codis/db_xxx/proxy"
     * @param poolConfig
     *            same as JedisPool
     */
    public RoundRobinJedisPool(CuratorFramework curatorClient, boolean closeCurator, String zkPath,
            JedisPoolConfig poolConfig) {
        this(curatorClient, closeCurator, zkPath, poolConfig, JEDIS_POOL_TIMEOUT_UNSET);
    }

    /**
     * Create a RoundRobinJedisPool.
     * 
     * @param curatorClient
     *            We will start it if it has not started yet.
     * @param closeCurator
     *            Whether to close the curatorClient passed in when close.
     * @param zkPath
     *            the codis proxy dir on ZooKeeper. e.g.
     *            "/zk/codis/db_xxx/proxy"
     * @param poolConfig
     *            same as JedisPool
     * @param timeout
     *            timeout of JedisPool
     */
    public RoundRobinJedisPool(CuratorFramework curatorClient, boolean closeCurator, String zkPath,
            JedisPoolConfig poolConfig, int timeout) {
        this.poolConfig = poolConfig;
        this.timeout = timeout;
        this.curatorClient = curatorClient;
        this.closeCurator = closeCurator;
        watcher = new PathChildrenCache(curatorClient, zkPath, true);
        watcher.getListenable().addListener(new PathChildrenCacheListener() {

            @Override
            public void childEvent(CuratorFramework client, PathChildrenCacheEvent event)
                    throws Exception {
                StringBuilder sb = new StringBuilder("zookeeper event received: type=")
                        .append(event.getType());
                if (event.getData() != null) {
                    ChildData data = event.getData();
                    sb.append(", path=").append(data.getPath()).append(", stat=")
                            .append(data.getStat());
                }
                LOG.info(sb.toString());
                if (RESET_TYPES.contains(event.getType())) {
                    resetPools();
                }
            }
        });
        // we need to get the initial data so client must be started
        if (curatorClient.getState() == CuratorFrameworkState.LATENT) {
            curatorClient.start();
        }
        try {
            watcher.start(StartMode.BUILD_INITIAL_CACHE);
        } catch (Exception e) {
            throw new JedisException(e);
        }
        resetPools();
    }

    private void resetPools() {
        ImmutableList<PooledObject> pools = this.pools;
        Map<String, PooledObject> addr2Pool = Maps.newHashMapWithExpectedSize(pools.size());
        for (PooledObject pool: pools) {
            addr2Pool.put(pool.addr, pool);
        }
        ImmutableList.Builder<PooledObject> builder = ImmutableList.builder();
        for (ChildData childData: watcher.getCurrentData()) {
            try {
                JsonNode proxyInfo = MAPPER.readTree(childData.getData());
                if (!CODIS_PROXY_STATE_ONLINE.equals(proxyInfo.get(JSON_NAME_CODIS_PROXY_STATE)
                        .asText())) {
                    continue;
                }
                String addr = proxyInfo.get(JSON_NAME_CODIS_PROXY_ADDR).asText();
                PooledObject pool = addr2Pool.remove(addr);
                if (pool == null) {
                    LOG.info("Add new proxy: " + addr);
                    String[] hostAndPort = addr.split(":");
                    String host = hostAndPort[0];
                    int port = Integer.parseInt(hostAndPort[1]);
                    if (timeout == JEDIS_POOL_TIMEOUT_UNSET) {
                        pool = new PooledObject(addr, new JedisPool(poolConfig, host, port));
                    } else {
                        pool = new PooledObject(addr,
                                new JedisPool(poolConfig, host, port, timeout));
                    }
                }
                builder.add(pool);
            } catch (Exception e) {
                LOG.warn("parse " + childData.getPath() + " failed", e);
            }
        }
        this.pools = builder.build();
        for (PooledObject pool: addr2Pool.values()) {
            LOG.info("Remove proxy: " + pool.addr);
            pool.pool.close();
        }
    }

    @Override
    public Jedis getResource() {
        ImmutableList<PooledObject> pools = this.pools;
        if (pools.isEmpty()) {
            throw new JedisException("Proxy list empty");
        }
        for (;;) {
            int current = nextIdx.get();
            int next = current >= pools.size() - 1 ? 0 : current + 1;
            if (nextIdx.compareAndSet(current, next)) {
                return pools.get(next).pool.getResource();
            }
        }
    }

    @Override
    public void close() {
        try {
            Closeables.close(watcher, true);
        } catch (IOException e) {
            LOG.fatal("IOException should not have been thrown", e);
        }
        if (closeCurator) {
            curatorClient.close();
        }
        List<PooledObject> pools = this.pools;
        this.pools = ImmutableList.of();
        for (PooledObject pool: pools) {
            pool.pool.close();
        }
    }
}
