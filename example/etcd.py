#!/usr/bin/env python3

from utils import *

import atexit
import json
import datetime


class Etcd(Process):

    def __init__(self):
        self.logfile = "etcd.log"
        self.command = "etcd"
        Process.__init__(self, self.command, self.logfile)

        dict = {"pid": self.proc.pid}
        print("    >> etcd = " + json.dumps(dict, sort_keys=True))


if __name__ == "__main__":
    children = []
    atexit.register(kill_all, children)

    children.append(Etcd())

    check_alive(children, 3)

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
