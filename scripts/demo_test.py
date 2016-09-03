#!/usr/bin/env python3

import time
import atexit
import subprocess
import datetime
import shutil
import os

children = []

def killall():
    global children
    for p in children:
        p.exit()
    children = []

def checkall(seconds=0):
    if seconds != 0:
        time.sleep(seconds)
    global children
    for p in children:
        if not p.is_running():
            message = "process lost - {}".format(p.command)
            raise Exception(message)


atexit.register(killall)


class Process:

    global children

    def __init__(self, command, logfile=None):
        self.command = command
        if logfile is not None:
            self.logfile = open(logfile, "wb+")
        try:
            self.proc = subprocess.Popen(self.command.split(), stderr=subprocess.STDOUT, stdout=self.logfile)
        except Exception:
            print("run command failed: {}".format(self.command))
            raise
        children.append(self)

    def is_running(self):
        try:
            self.proc.wait(0)
        except subprocess.TimeoutExpired:
            pass
        return self.proc.returncode is None

    def exit(self):
        if self.is_running():
            self.proc.kill()
        if self.logfile is not None:
            self.logfile.close()

    def wait(self):
        self.proc.wait()


class EtcdServer(Process):

    def __init__(self):
        logfile = "etcd.log"
        command = "etcd"
        Process.__init__(self, command, logfile)


class CodisServer(Process):

    def __init__(self, port, master_port=None, requirepass=None):
        self.config = self._open_config(port, master_port, requirepass)
        self.port = port

        logfile = "redis-{}.log".format(port)
        command = "codis-server {}".format(self.config)
        Process.__init__(self, command, logfile)

    def _open_config(self, port, master_port=None, requirepass=None):
        config = 'redis-{}.conf'.format(port)
        with open(config, "w+") as f:
            f.write('port {}\n'.format(port))
            f.write('save ""\n')
            f.write('dbfilename "{}.rdb"\n'.format(port))
            if master_port is not None:
                f.write('slaveof 127.0.0.1 {}\n'.format(master_port))
            if requirepass is not None:
                f.write('requirepass {}\n'.format(requirepass))
        return config


class CodisSentinel(Process):

    def __init__(self, port):
        self.config = self._open_config(port)
        self.port = port

        logfile = "sentinel-{}.log".format(port)
        command = "codis-server {} --sentinel".format(self.config)
        Process.__init__(self, command, logfile)

    def _open_config(self, port):
        config = 'sentinel-{}.conf'.format(port)
        with open(config, "w+") as f:
            f.write('port {}'.format(port))
        return config


class CodisProxy(Process):

    def __init__(self, admin_port, proxy_port, product_name, product_auth=None):
        self.config = self._open_config(admin_port, proxy_port, product_name, product_auth)
        self.admin_port = admin_port
        self.proxy_port = proxy_port
        self.product_name = product_name
        self.product_auth = product_auth

        logfile = "proxy-{}.log".format(proxy_port)
        command = "codis-proxy -c {} --etcd 127.0.0.1:2379".format(self.config)
        Process.__init__(self, command, logfile)

    def _open_config(self, admin_port, proxy_port, product_name, product_auth=None):
        config = 'proxy-{}.toml'.format(proxy_port)
        with open(config, "w+") as f:
            f.write('product_name = "{}"\n'.format(product_name))
            if product_auth is not None:
                f.write('product_auth = "{}"\n'.format(product_auth))
            f.write('proto_type = "tcp4"\n')
            f.write('admin_addr = "0.0.0.0:{}"\n'.format(admin_port))
            f.write('proxy_addr = "0.0.0.0:{}"\n'.format(proxy_port))
            f.write('proxy_datacenter = "localhost"\n')
            f.write('proxy_heap_placeholder = "0"\n')
            f.write('proxy_max_offheap_size = "0"\n')
        return config


class CodisDashboard(Process):

    def __init__(self, admin_port, product_name, product_auth=None):
        self.config = self._open_config(admin_port, product_name, product_auth)
        self.admin_port = admin_port
        self.product_name = product_name
        self.product_auth = product_auth

        logfile = "dashboard-{}.log".format(admin_port)
        command = "codis-dashboard -c {}".format(self.config)
        Process.__init__(self, command, logfile)

    def _open_config(self, admin_port, product_name, product_auth=None):
        config = 'dashboard-{}.toml'.format(admin_port)
        with open(config, "w+") as f:
            f.write('coordinator_name = "etcd"\n')
            f.write('coordinator_addr = "127.0.0.1:2379"\n')
            f.write('product_name = "{}"\n'.format(product_name))
            if product_auth is not None:
                f.write('product_auth = "{}"\n'.format(product_auth))
            f.write('admin_addr = "0.0.0.0:{}"\n'.format(admin_port))
        return config


class CodisFe(Process):

    def __init__(self, port, assets):
        self.port = port

        logfile = "fe-{}.log".format(port)
        command = "codis-fe --etcd 127.0.0.1:2379 --listen 0.0.0.0:{} --assets-dir={}".format(self.port, assets)
        Process.__init__(self, command, logfile)


def codis_admin_dashboard(port, args=None):
    command = "codis-admin --dashboard 127.0.0.1:{}".format(port)
    if args is not None:
        command += " " + args
    subprocess.call(command.split())


def codis_admin_proxy(admin_port, args=None):
    command = "codis-admin --proxy 127.0.0.1:{}".format(admin_port)
    if args is not None:
        command += " " + args
    subprocess.call(command.split())


if __name__ == "__main__":
    os.environ["PATH"] += os.pathsep + os.getcwd()
    os.environ["PATH"] += os.pathsep + os.path.abspath(os.path.join(os.getcwd(), "..", "bin"))
    shutil.rmtree("tmp", ignore_errors=True)
    os.mkdir("tmp")
    os.chdir("tmp")

    EtcdServer()
    print("init etcd, done")

    for i in range(0, 4):
        CodisServer(16379+i)
    for i in range(0, 4):
        CodisServer(17379+i, master_port=16379+i)
    print("init codis-server, done")

    for i in range(0, 5):
        CodisSentinel(26379+i)
    print("init codis-sentinel, done")

    checkall(3)
    print("checkall, done")

    d = CodisDashboard(18080, "demo-test")
    print("init codis-dashboard, done")

    checkall(3)
    print("checkall, done")

    for i in range(0, 4):
        CodisProxy(11080+i, 19000+i, "demo-test")
    print("init codis-proxy, done")

    CodisFe(8080, "../../cmd/fe/assets")
    print("init codis-fe, done")

    checkall(3)
    print("checkall, done")

    for i in range(0, 4):
        codis_admin_dashboard(d.admin_port, "--create-group --gid={}".format(i+1))
        codis_admin_dashboard(d.admin_port, "--group-add --gid={} --addr=127.0.0.1:{} --datacenter=localhost".format(i+1, 16379+i))
        codis_admin_dashboard(d.admin_port, "--group-add --gid={} --addr=127.0.0.1:{} --datacenter=localhost".format(i+1, 17379+i))
    print("create groups, done")

    for i in range(0, 5):
        codis_admin_dashboard(d.admin_port, "--sentinel-add --addr=127.0.0.1:{}".format(26379+i))
    print("add sentinels, done")

    codis_admin_dashboard(d.admin_port, "--slot-action --interval=100")
    codis_admin_dashboard(d.admin_port, "--sentinel-resync")
    for i in range(0, 4):
        gid = i + 1
        beg, end = i * 256, (i + 1) * 256 - 1
        codis_admin_dashboard(d.admin_port, "--slots-assign --beg={} --end={} --gid={} --confirm".format(beg, end, gid))

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
