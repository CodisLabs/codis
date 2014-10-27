FROM debian:latest

# upgrade & install required packages
RUN apt-get update && apt-get upgrade -y
RUN apt-get install -y \
    openssh-server bash vim gcc make bzip2 curl wget

RUN mkdir -p /codis

# build patched redis-2.8.13
ENV TMPBUILD /tmp/build
ADD redis-2.8.13 ${TMPBUILD}
WORKDIR ${TMPBUILD}
RUN make distclean && cd deps && ./update-jemalloc.sh 3.6.0
WORKDIR ${TMPBUILD}
RUN make -j && cd src && cp redis-server redis-cli /codis
WORKDIR /
RUN rm -rf ${TMPBUILD}

# set root's password=root
RUN echo 'root:root' | chpasswd

# create user codis, set codis' password=codis
RUN groupadd -r codis && useradd -r -g codis codis -s /bin/bash -d /codis
RUN echo 'codis:codis' | chpasswd

# copy default redis.conf, use default port 6379
ADD test/conf/6379.conf /codis/redis.conf
EXPOSE 6379

# set sshd as default entrypoint
RUN mkdir -p /var/run/sshd
ENTRYPOINT ["/usr/sbin/sshd", "-D"]
EXPOSE 22

RUN chown -R codis:codis /codis
