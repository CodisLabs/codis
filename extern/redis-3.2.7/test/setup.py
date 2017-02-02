#!/usr/bin/env python3

from utils import *

import atexit
import json
import datetime


class Redis(Process):

    def __init__(self, port, master_port=None, requirepass=None):
        self.config = self._open_config(port, master_port, requirepass)
        self.port = port

        self.logfile = "{}.log".format(port)
        self.command = "valgrind --leak-check=full -v redis-server {}".format(self.config)
        Process.__init__(self, self.command, self.logfile)

        dict = {"port": port, "pid": self.proc.pid}
        print("    >> redis.server = " + json.dumps(dict, sort_keys=True))

    @staticmethod
    def _open_config(port, master_port=None, requirepass=None):
        config = '{}.conf'.format(port)
        with open(config, "w+") as f:
            f.write('port {}\n'.format(port))
            f.write('save ""\n')
            f.write('dbfilename "{}.rdb"\n'.format(port))
            if master_port is not None:
                f.write('slaveof 127.0.0.1 {}\n'.format(master_port))
            if requirepass is not None:
                f.write('masterauth {}\n'.format(requirepass))
                f.write('requirepass {}\n'.format(requirepass))
            f.write('protected-mode no\n')
        return config


if __name__ == "__main__":
    children = []
    atexit.register(kill_all, children)

    passwd = None

    for port in range(2000, 2002):
        children.append(Redis(port, requirepass=passwd))
        children.append(Redis(port+10, master_port=port, requirepass=passwd))

    check_alive(children, 3)

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
