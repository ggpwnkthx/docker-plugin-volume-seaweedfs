#!/bin/sh
PLUGIN_NAME=docker-plugin-volume-seaweedfs

docker run -it --rm -v $(pwd):/src/app -w /src/app/src golang:alpine go build -o ../$PLUGIN_NAME

docker build -t $PLUGIN_NAME .
CONTAINER_ID=$(docker create $PLUGIN_NAME true)
mkdir -p plugin/rootfs
docker export "$CONTAINER_ID" | tar -x -C plugin/rootfs
docker rm -vf "$CONTAINER_ID"
docker rmi $PLUGIN_NAME
docker plugin create $PLUGIN_NAME ./plugin
