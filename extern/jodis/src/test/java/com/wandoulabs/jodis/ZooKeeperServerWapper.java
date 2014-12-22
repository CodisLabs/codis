/**
 * @(#)ZooKeeperServerWapper.java, 2014-11-30. 
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

import java.io.File;
import java.io.IOException;

import org.apache.zookeeper.server.ServerCnxnFactory;
import org.apache.zookeeper.server.ZooKeeperServer;
import org.apache.zookeeper.server.persistence.FileTxnSnapLog;

/**
 * @author Apache9
 */
public class ZooKeeperServerWapper {

    private volatile ServerCnxnFactory cnxnFactory;

    private volatile ZooKeeperServer zkServer;

    private volatile FileTxnSnapLog ftxn;

    private int port;

    private File baseDir;

    public ZooKeeperServerWapper(int port, File baseDir) throws Exception {
        this.port = port;
        this.baseDir = baseDir;
    }

    public void start() throws IOException, InterruptedException {
        ZooKeeperServer zkServer = new ZooKeeperServer();
        FileTxnSnapLog ftxn = new FileTxnSnapLog(
                new File(baseDir, "zookeeper"), new File(baseDir, "zookeeper"));
        zkServer.setTxnLogFactory(ftxn);
        zkServer.setTickTime(1000);
        ServerCnxnFactory cnxnFactory = ServerCnxnFactory.createFactory(port,
                100);
        cnxnFactory.startup(zkServer);
        this.cnxnFactory = cnxnFactory;
        this.zkServer = zkServer;
        this.ftxn = ftxn;
    }

    public void stop() throws IOException {
        cnxnFactory.shutdown();
        ftxn.close();
    }

    public boolean isRunning() {
        if (zkServer == null) {
            return false;
        } else {
            return zkServer.isRunning();
        }
    }
}
