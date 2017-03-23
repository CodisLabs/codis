FROM golang:1.7.3

RUN apt-get update
RUN apt-get install -y autoconf

<<<<<<< HEAD
# Add codis
Add . /go/src/github.com/CodisLabs/codis/
WORKDIR /go/src/github.com/CodisLabs/codis/

# Install dependency
RUN ./bootstrap.sh
WORKDIR /go/src/github.com/CodisLabs/codis/sample
=======
ENV GOPATH /gopath
ENV CODIS  ${GOPATH}/src/github.com/CodisLabs/codis
ENV PATH   ${GOPATH}/bin:${PATH}:${CODIS}/bin
COPY . ${CODIS}

RUN make -C ${CODIS} distclean
RUN make -C ${CODIS} build-all
>>>>>>> CodisLabs/release3.1

WORKDIR /codis
