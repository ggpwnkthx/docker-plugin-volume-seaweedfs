#!/bin/sh
SCOPE_USER=ggpwnkthx
PLUGIN_NAME=docker-plugin-volume-seaweedfs

# Clean-up
docker plugin disable $SCOPE_USER/$PLUGIN_NAME
docker plugin rm $SCOPE_USER/$PLUGIN_NAME

# Build
docker build -t $SCOPE_USER/$PLUGIN_NAME . && \
CONTAINER_ID=$(docker create $SCOPE_USER/$PLUGIN_NAME true) && \
mkdir -p plugin/rootfs && \
docker export "$CONTAINER_ID" | tar -x -C plugin/rootfs && \
docker rm -vf "$CONTAINER_ID" && \
#docker rmi $SCOPE_USER/$PLUGIN_NAME && \
docker plugin create $SCOPE_USER/$PLUGIN_NAME ./plugin && \
docker plugin enable $SCOPE_USER/$PLUGIN_NAME