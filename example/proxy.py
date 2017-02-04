#!/usr/bin/env python3

from utils import *

import atexit
import json
import datetime


class CodisProxy(Process):

    def __init__(self, admin_port, proxy_port, product_name, product_auth=None):
        self.config = self._open_config(admin_port, proxy_port, product_name, product_auth)
        self.admin_port = admin_port
        self.proxy_port = proxy_port
        self.product_name = product_name
        self.product_auth = product_auth

        self.logfile = "proxy-{}.log".format(proxy_port)
        self.command = "codis-proxy -c {} --filesystem rootfs".format(self.config)
        Process.__init__(self, self.command, self.logfile)

        dict = {"admin_port": admin_port, "proxy_port": proxy_port, "pid": self.proc.pid}
        print("    >> codis.proxy = " + json.dumps(dict, sort_keys=True))

    @staticmethod
    def _open_config(admin_port, proxy_port, product_name, product_auth=None):
        config = 'proxy-{}.toml'.format(proxy_port)
        with open(config, "w+") as f:
            f.write('product_name = "{}"\n'.format(product_name))
            if product_auth is not None:
                f.write('product_auth = "{}"\n'.format(product_auth))
            f.write('proto_type = "tcp4"\n')
            f.write('admin_addr = ":{}"\n'.format(admin_port))
            f.write('proxy_addr = ":{}"\n'.format(proxy_port))
            f.write('proxy_datacenter = "localhost"\n')
            f.write('proxy_heap_placeholder = "0"\n')
            f.write('proxy_max_offheap_size = "0"\n')
            f.write('backend_ping_period = "5s"\n')
            f.write('backend_recv_bufsize = "128kb"\n')
            f.write('backend_recv_timeout = "30s"\n')
            f.write('backend_send_bufsize = "128kb"\n')
            f.write('backend_send_timeout = "30s"\n')
            f.write('backend_max_pipeline = 1024\n')
            f.write('backend_primary_only = false\n')
            f.write('backend_keepalive_period = "75s"\n')
            f.write('session_recv_bufsize = "128kb"\n')
            f.write('session_recv_timeout = "30m"\n')
            f.write('session_send_bufsize = "64kb"\n')
            f.write('session_send_timeout = "30s"\n')
            f.write('session_max_pipeline = 1024\n')
            f.write('session_keepalive_period = "75s"\n')
            f.write('session_break_on_failure = false\n')
        return config


if __name__ == "__main__":
    children = []
    atexit.register(kill_all, children)

    product_name = "demo-test"
    product_auth = None

    for i in range(0, 4):
        CodisProxy(11080+i, 19000+i, product_name, product_auth)

    check_alive(children, 3)

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
