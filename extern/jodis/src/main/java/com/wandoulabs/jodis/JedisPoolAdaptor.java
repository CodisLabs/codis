/**
 * @(#)JedisPoolAdaptor.java, 2014-12-2. 
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

import java.net.URI;

import org.apache.commons.pool2.impl.GenericObjectPoolConfig;

import redis.clients.jedis.JedisPool;

/**
 * Adaptor of JedisPool to make writing testcase easier.
 * 
 * @author Apache9
 */
public class JedisPoolAdaptor extends JedisPool implements JedisResourcePool {

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, String host, int port, int timeout,
            String password, int database, String clientName) {
        super(poolConfig, host, port, timeout, password, database, clientName);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, String host, int port, int timeout,
            String password, int database) {
        super(poolConfig, host, port, timeout, password, database);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, String host, int port, int timeout,
            String password) {
        super(poolConfig, host, port, timeout, password);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, String host, int port, int timeout) {
        super(poolConfig, host, port, timeout);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, String host, int port) {
        super(poolConfig, host, port);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, String host) {
        super(poolConfig, host);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, URI uri, int timeout) {
        super(poolConfig, uri, timeout);
    }

    public JedisPoolAdaptor(GenericObjectPoolConfig poolConfig, URI uri) {
        super(poolConfig, uri);
    }

    public JedisPoolAdaptor(String host, int port) {
        super(host, port);
    }

    public JedisPoolAdaptor(String host) {
        super(host);
    }

    public JedisPoolAdaptor(URI uri, int timeout) {
        super(uri, timeout);
    }

    public JedisPoolAdaptor(URI uri) {
        super(uri);
    }

}
