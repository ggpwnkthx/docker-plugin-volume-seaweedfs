#!/bin/sh
cat <<EOF > /usr/local/etc/haproxy/haproxy.cfg
global
    log stdout local0 debug
defaults
    log global
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
listen http_socket
    mode http
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/http.sock
    server http_filer filer:8888 check
listen grpc_socket
    mode tcp
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/grpc.sock
    server grpc_filer filer:18888
EOF

/docker-entrypoint.sh "haproxy" "-f" "/usr/local/etc/haproxy/haproxy.cfg"