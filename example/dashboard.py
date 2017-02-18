#!/usr/bin/env python3

from utils import *

import atexit
import json
import datetime


class CodisDashboard(Process):

    def __init__(self, admin_port, product_name, product_auth=None):
        self.config = self._open_config(admin_port, product_name, product_auth)
        self.admin_port = admin_port
        self.product_name = product_name
        self.product_auth = product_auth

        self.logfile = "dashboard-{}.log".format(admin_port)
        self.command = "codis-dashboard -c {}".format(self.config)
        Process.__init__(self, self.command, self.logfile)

        dict = {"admin_port": admin_port, "pid": self.proc.pid}
        print("    >> codis.dashboard = " + json.dumps(dict, sort_keys=True))

    @staticmethod
    def _open_config(admin_port, product_name, product_auth=None):
        config = 'dashboard-{}.toml'.format(admin_port)
        with open(config, "w+") as f:
            f.write('coordinator_name = "filesystem"\n')
            f.write('coordinator_addr = "rootfs"\n')
            f.write('product_name = "{}"\n'.format(product_name))
            if product_auth is not None:
                f.write('product_auth = "{}"\n'.format(product_auth))
            f.write('admin_addr = ":{}"\n'.format(admin_port))
            f.write('migration_method = "semi-async"\n')
            f.write('migration_async_maxbulks = 200\n')
            f.write('migration_async_maxbytes = "32mb"\n')
            f.write('migration_async_numkeys = 100\n')
            f.write('migration_timeout = "30s"\n')
            f.write('sentinel_quorum = 2\n')
            f.write('sentinel_parallel_syncs = 1\n')
            f.write('sentinel_down_after = "5s"\n')
            f.write('sentinel_failover_timeout = "10m"\n')
            path = os.getcwd()
            f.write('sentinel_notification_script = "{}"\n'.format(os.path.join(path, "sentinel_notify.sh")))
            f.write('sentinel_client_reconfig_script = "{}"\n'.format(os.path.join(path, "sentinel_reconfig.sh")))
        return config


if __name__ == "__main__":
    children = []
    atexit.register(kill_all, children)

    product_name = "demo-test"
    product_auth = None

    children.append(CodisDashboard(18080, product_name, product_auth))

    check_alive(children, 3)

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
