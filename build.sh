#!/bin/sh
SCOPE_USER=ggpwnkthx
PLUGIN_NAME=docker-plugin-volume-seaweedfs

docker build -t $PLUGIN_NAME .
CONTAINER_ID=$(docker create $PLUGIN_NAME true)
mkdir -p plugin/rootfs
docker export "$CONTAINER_ID" | tar -x -C plugin/rootfs
docker rm -vf "$CONTAINER_ID"
docker rmi $PLUGIN_NAME
docker plugin create $SCOPE_USER/$PLUGIN_NAME ./plugin
