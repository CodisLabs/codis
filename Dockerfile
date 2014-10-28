FROM golang:1.3

# upgrade & install required packages
RUN apt-get update && apt-get upgrade -y
RUN apt-get install -y \
    openssh-server bash vim golang

# build proxy & cconfig
ENV GOPATH /tmp/gopath
RUN go get github.com/c4pt0r/cfg
RUN go get github.com/garyburd/redigo/redis
RUN go get github.com/juju/errgo
RUN go get github.com/juju/errors
RUN go get github.com/juju/loggo
RUN go get github.com/ngaut/go-zookeeper/zk
RUN go get github.com/ngaut/gostats
RUN go get github.com/ngaut/logging
RUN go get github.com/ngaut/pools
RUN go get github.com/ngaut/resp
RUN go get github.com/ngaut/sync2
RUN go get github.com/codegangsta/martini-contrib/binding
RUN go get github.com/go-martini/martini
RUN go get github.com/martini-contrib/cors
RUN go get github.com/nu7hatch/gouuid
RUN go get github.com/docopt/docopt-go
ADD pkg ${GOPATH}/src/github.com/wandoulabs/codis/pkg

RUN mkdir -p /codis

ENV TMPBUILD /tmp/build
ADD cmd ${TMPBUILD}
WORKDIR ${TMPBUILD}
RUN go build -a -o /codis/bin/cconfig ./cconfig/
RUN go build -a -o /codis/bin/proxy ./proxy/
ADD cmd/cconfig/assets /codis/bin/assets
RUN rm -rf ${TMPBUILD}
ADD deploy /codis/deploy

# set root's password=root
RUN echo 'root:root' | chpasswd

# create user codis, set codis' password=codis
RUN groupadd -r codis && useradd -r -g codis codis -s /bin/bash -d /codis
RUN echo "codis:codis" | chpasswd

EXPOSE 19000
EXPOSE 11000
EXPOSE 8087

# set sshd as default entrypoint
RUN mkdir -p /var/run/sshd
ENTRYPOINT ["/usr/sbin/sshd", "-D"]
EXPOSE 22

RUN chown -R codis:codis /codis
