#!/bin/sh

make clean

echo "downloading dependcies, it may take a few minutes..."
# Test godep install, steal it from LedisDB project :P
godep path > /dev/null 2>&1
if [ "$?" = 0 ]; then
    GOPATH=`godep path`:$GOPATH
    godep restore
    make || exit $?
    make gotest
    exit 0
fi

go get -u github.com/coreos/etcd/error
go get -u github.com/coreos/go-etcd/etcd
go get -u github.com/bsm/redeo
go get -u github.com/c4pt0r/cfg
go get -u github.com/garyburd/redigo/redis
go get -u github.com/juju/errgo
go get -u github.com/juju/errors
go get -u github.com/juju/loggo
go get -u github.com/ngaut/go-zookeeper/zk
go get -u github.com/ngaut/gostats
go get -u github.com/ngaut/logging
go get -u github.com/ngaut/zkhelper
go get -u github.com/ngaut/pools
go get -u github.com/ngaut/deadline
go get -u github.com/ngaut/resp
go get -u github.com/ngaut/tokenlimiter
go get -u github.com/ngaut/sync2
go get -u github.com/codegangsta/martini-contrib/binding
go get -u github.com/go-martini/martini
go get -u github.com/martini-contrib/cors
go get -u github.com/nu7hatch/gouuid
go get -u github.com/docopt/docopt-go
go get -u github.com/cupcake/rdb
go get -u github.com/alicebob/miniredis

make || exit $?
make gotest
