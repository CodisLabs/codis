#!/bin/sh

rm bin/*.log
cd ext/redis-2.8.13 && make && cd -

go get github.com/c4pt0r/cfg
go get github.com/garyburd/redigo/redis
go get github.com/juju/errgo
go get github.com/juju/errors
go get github.com/juju/loggo
go get github.com/ngaut/go-zookeeper/zk
go get github.com/ngaut/gostats
go get github.com/ngaut/logging
go get github.com/ngaut/pools
go get github.com/ngaut/resp
go get github.com/ngaut/sync2
go get github.com/codegangsta/martini-contrib/binding
go get github.com/go-martini/martini
go get github.com/martini-contrib/cors
go get github.com/nu7hatch/gouuid
go get github.com/docopt/docopt-go
go get github.com/cupcake/rdb
go get github.com/alicebob/miniredis

make



