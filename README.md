https://hub.docker.com/r/ggpwnkthx/docker-plugin-volume-seaweedfs

This plugin uses Unix sockets to communicate with the SeaweedFS Filer nodes. It also uses the Filer as a proxy to access the SeaweedFS Volume nodes. That way all traffic routes through the Unix sockets. The reason Unix sockets are used is to keep the mounting process flexible enough to be used in a Docker Swarm environment. Docker Volume Plugins currently are not able to attach to an overlay network.

Each Filer requires 2 Unix sockets:
```
/var/lib/docker/plugins/seaweedfs/[filer alias]/http.sock
/var/lib/docker/plugins/seaweedfs/[filer alias]/grpc.sock
```

We can create a globally replicated service that is attached to the overlay network that the SeaweedFS Filer nodes are also attached to, and use that service to run ```socat``` to establish a Unix socket that relays traffic to the SeaweedFS Filer nodes. By doing it this way, we can also take advantage of Docker's built-in load balancing. See below for an example.
```
OVERLAY_NETWORK="stack_seaweedfs"
FILER_ALIAS="filer"
FILER_PORT="8888"

# HTTP Socket
docker service create \
    --mode "global" \
    --mount "type=bind,source=/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS,destination=/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS" \
    --network "name=$OVERLAY_NETWORK" \
    --restart-condition "any" \
    alpine/socat \
        unix-l:/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS/http.sock,fork \
        tcp-connect:$FILER_ALIAS:$FILER_PORT

# gRPC Socket
docker service create \
    --mode "global" \
    --mount "type=bind,source=/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS,destination=/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS" \
    --network "name=$OVERLAY_NETWORK" \
    --restart-condition "any" \
    alpine/socat \
        unix-l:/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS/grpc.sock,fork \
        tcp-connect:$FILER_ALIAS:$(($FILER_PORT + 10000))
```

Of course, the prerequisite here is that each node in the Docker Swarm would need to already have the ```/var/lib/docker/plugins/seaweedfs/$FILER_ALIAS``` directory. A work around for this is to use a ```global-job``` similar to the example below.

```
FILER_ALIAS="filer"
docker service create \
    --mode "global-job" \
    --mount "type=bind,source=/var/lib/docker/plugins/,destination=/var/lib/docker/plugins/" \
    alpine:3 \
        mkdir -p /var/lib/docker/plugins/seaweedfs/$FILER_ALIAS
```

This plugin will automatically create the necessary TCP listeners using random ports for each SeaweedFS Filer and relay data to the respective Unix sockets. Although multiple Docker Volumes can be created using the same SeaweedFS Filer, only one HTTP and one gRPC relay will be initiated per SeaweedFS Filer. Perhaps in the future SeaweedFS will support mounting using Unix sockets directly.

To create a Docker Volume using this plugin, see the example below.

```docker volume create --driver ggpwnkthx/docker-plugin-volume-seaweedfs --opt filer=filer_alias volume_name```

In the above example, this plugin will use the Unix sockets found in the ```/var/lib/docker/plugins/seaweedfs/filer_alias``` directory to mount the root directory hosted by that SeaweedFS instance to the Docker Volume named ```volume_name```.

Using the ```--opt``` parameter, we can passthru options to the ```weed mount``` command used in this plugin. Similar to the example above, the example below shows how to create a Docker Volume that mounts a specific directory hosted by the SeaweedFS instance instead of the root directory.

```docker volume create --driver ggpwnkthx/docker-plugin-volume-seaweedfs --opt filer=filer_alias --opt filer.path=/path/you/want/mounted volume_name```
