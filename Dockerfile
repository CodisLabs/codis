FROM golang:1.4
MAINTAINER goroutine@126.com

RUN apt-get update -y

# Add codis
Add . /go/src/github.com/wandoulabs/codis/
WORKDIR /go/src/github.com/wandoulabs/codis/

# Install dependency
RUN ./bootstrap.sh
WORKDIR /go/src/github.com/wandoulabs/codis/sample

# Expose ports
EXPOSE 19000
EXPOSE 11000
EXPOSE 18087

CMD ./startall.sh && tail -f log/*
