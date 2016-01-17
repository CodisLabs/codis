FROM golang:1.4

RUN apt-get update
RUN apt-get install -y vim bash golang
RUN apt-get clean
RUN rm -rf /var/lib/apt/lists/*

ENV GOPATH /gopath
ENV CODIS  ${GOPATH}/src/github.com/CodisLabs/codis
ENV PATH   ${GOPATH}/bin:${PATH}:${CODIS}/bin
COPY . ${CODIS}

RUN make -C ${CODIS} distclean
RUN make -C ${CODIS} build-all

WORKDIR /codis
