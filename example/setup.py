#!/usr/bin/env python3

from server import *
from sentinel import *
from dashboard import *
from proxy import *
from fe import *


def codis_admin_dashboard(admin_port, args=None):
    command = "codis-admin --dashboard 127.0.0.1:{}".format(admin_port)
    if args is not None:
        command += " " + args
    return do_command(command)


def codis_admin_proxy(admin_port, args=None):
    command = "codis-admin --proxy 127.0.0.1:{}".format(admin_port)
    if args is not None:
        command += " " + args
    return do_command(command)

if __name__ == "__main__":
    children = []
    atexit.register(kill_all, children)

    product_name = "demo-test"
    product_auth = None

    # step 1. codis-server & codis-sentinel

    # codis-server [master 2000+i <== following == 3000+i slave]
    for i in range(0, 4):
        children.append(CodisServer(2000 + i, requirepass=product_auth))
        children.append(CodisServer(3000 + i, 2000 + i, requirepass=product_auth))

    for i in range(0, 5):
        children.append(CodisSentinel(20000 + i))

    check_alive(children, 1)
    print("[OK] setup codis-server & codis-sentinel")

    # step 2. setup codis-fe & codis-dashboard & codis-proxy

    children.append(CodisFE(8080, "../cmd/fe/assets"))
    children.append(CodisDashboard(18080, product_name, product_auth))

    for i in range(0, 4):
        children.append(CodisProxy(11080 + i, 19000 + i, product_name, product_auth))

    check_alive(children, 3)
    print("[OK] setup codis-fe & codis-dashboard & codis-proxy")

    # step3: init slot-mappings

    for i in range(0, 4):
        gid = i + 1
        codis_admin_dashboard(18080, "--create-group --gid={}".format(gid))
        codis_admin_dashboard(18080, "--group-add --gid={} --addr=127.0.0.1:{} --datacenter=localhost".format(gid, 2000+i))
        codis_admin_dashboard(18080, "--group-add --gid={} --addr=127.0.0.1:{} --datacenter=localhost".format(gid, 3000+i))
        beg, end = i * 256, (i + 1) * 256 - 1
        codis_admin_dashboard(18080, "--slots-assign --beg={} --end={} --gid={} --confirm".format(beg, end, gid))
        codis_admin_dashboard(18080, "--resync-group --gid={}".format(gid))

    for i in range(0, 5):
        codis_admin_dashboard(18080, "--sentinel-add --addr=127.0.0.1:{}".format(20000+i))

    codis_admin_dashboard(18080, "--slot-action --interval=0")
    codis_admin_dashboard(18080, "--sentinel-resync")

    check_alive(children, 3)
    print("[OK] done & have fun!!!")

    while True:
        print(datetime.datetime.now())
        time.sleep(5)
