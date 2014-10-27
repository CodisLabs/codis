#!/bin/bash

docker rmi codis/redis
docker build --force-rm -t codis/redis .

# docker run --name "codis-redis" -h "codis-redis" -d -p 6022:22 -p 6079:6379 codis/redis
