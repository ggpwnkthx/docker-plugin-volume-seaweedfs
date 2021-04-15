#!/bin/sh
if [ -d /var/lib/docker/plugins/seaweedfs/$1 ]; then
    cat <<EOF >> /usr/local/etc/haproxy/haproxy.cfg
listen http_socket
    mode http
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/http.sock
    server http_filer filer:8888 check
listen grpc_socket
    mode tcp
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/grpc.sock
    server grpc_filer filer:18888
EOF
    haproxy -f /usr/local/etc/haproxy/haproxy.cfg
fi
echo "/var/lib/docker/plugins/seaweedfs/$1 not found"