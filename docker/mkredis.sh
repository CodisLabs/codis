#!/bin/bash

cd ../extern || exit $?

docker rmi codis/redis

cat > Dockerfile <<EOF
FROM debian:latest

# upgrade & install required packages
RUN apt-get update
RUN apt-get upgrade -y
RUN apt-get install -y \\
    openssh-server bash vim gcc make bzip2 curl wget

RUN echo 'root:root' | chpasswd

RUN mkdir -p /var/run/sshd
ENTRYPOINT ["/usr/sbin/sshd", "-D"]
EXPOSE 22

ENV HOMEDIR /codis
RUN mkdir -p \${HOMEDIR}

RUN groupadd -r codis && useradd -r -g codis codis -s /bin/bash -d \${HOMEDIR}
RUN echo 'codis:codis' | chpasswd

ENV BUILDDIR /tmp/codis
RUN mkdir -p \${BUILDDIR}

# copy & build redis source code
ADD redis-2.8.13 \${BUILDDIR}
WORKDIR \${BUILDDIR}/src
RUN make distclean
RUN make -j
RUN cp redis-server \${HOMEDIR}/codis-server
RUN cp redis-cli    \${HOMEDIR}/
RUN rm -rf \${BUILDDIR}
ADD redis-test/conf/6379.conf \${HOMEDIR}/redis.conf
EXPOSE 6379

RUN chown -R codis:codis \${HOMEDIR}
EOF

docker build --force-rm -t codis/redis . && rm -f Dockerfile

# docker run --name "codis-redis" -h "codis-redis" -d -p 6022:22 -p 6079:6379 codis/redis
