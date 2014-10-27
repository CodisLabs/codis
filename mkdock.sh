#!/bin/bash

docker rmi codis/proxy
docker build --force-rm -t codis/proxy .

# docker run --name "codis-proxy" -h "codis-proxy" -d -p 2022:22 -p 19000:19000 -p 11000:11000 -p 8087:8087 codis/proxy
