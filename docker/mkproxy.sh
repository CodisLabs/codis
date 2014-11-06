#!/bin/bash

cd .. || exit $?

docker rm -f codis-proxy
docker rmi codis/proxy

ADDGODEPS=`cat bootstrap.sh | grep "go  *get " | sed -e "s/^/RUN /g"`
if [ $? -ne 0 ]; then
    echo "generate ADDGODEPS failed"
    exit 1
fi

cat > Dockerfile <<EOF
FROM golang:1.3

# upgrade & install required packages
RUN apt-get update
RUN apt-get upgrade -y
RUN apt-get install -y \\
    openssh-server bash vim golang

RUN echo 'root:root' | chpasswd

RUN mkdir -p /var/run/sshd
ENTRYPOINT ["/usr/sbin/sshd", "-D"]
EXPOSE 22

ENV HOMEDIR /codis
RUN mkdir -p \${HOMEDIR}

RUN groupadd -r codis && useradd -r -g codis codis -s /bin/bash -d \${HOMEDIR}
RUN echo 'codis:codis' | chpasswd

ENV GOPATH /tmp/gopath
${ADDGODEPS}

ADD pkg \${GOPATH}/src/github.com/wandoulabs/codis/pkg

ENV BUILDDIR /tmp/codis
RUN mkdir -p \${BUILDDIR}

ADD cmd \${BUILDDIR}
WORKDIR \${BUILDDIR}
RUN go build -a -o \${HOMEDIR}/bin/codis-config ./cconfig/
RUN go build -a -o \${HOMEDIR}/bin/codis-proxy  ./proxy/
RUN rm -rf \${BUILDDIR}
ADD cmd/cconfig/assets \${HOMEDIR}/bin/assets
ADD sample \${HOMEDIR}/sample

WORKDIR \${HOMEDIR}
RUN ln -s sample/config.ini .

EXPOSE 19000
EXPOSE 11000
EXPOSE 18087

RUN chown -R codis:codis \${HOMEDIR}
EOF

docker build --force-rm -t codis/proxy . && rm -f Dockerfile

# docker run --name "codis-proxy" -h "codis-proxy" -d -p 2022:22 -p 19000:19000 -p 11000:11000 -p 18087:18087 codis/proxy
