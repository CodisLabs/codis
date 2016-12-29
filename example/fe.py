#!/usr/bin/env python3

from utils import *

import atexit
import json
import datetime


class CodisFE(Process):

    def __init__(self, port, assets):
        self.port = port

        self.logfile = "fe-{}.log".format(port)
        self.command = "codis-fe --filesystem rootfs --listen 0.0.0.0:{} --assets-dir={}".format(self.port, assets)
        Process.__init__(self, self.command, self.logfile)

        dict = {"pid": self.proc.pid, "assets": assets}
        print("    >> codis.fe = " + json.dumps(dict, sort_keys=True))


if __name__ == "__main__":
    children = []
    atexit.register(kill_all, children)

    children.append(CodisFE(8080, "../cmd/fe/assets"))

    check_alive(children, 3)

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
