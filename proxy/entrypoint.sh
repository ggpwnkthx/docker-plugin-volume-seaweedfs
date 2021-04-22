#!/bin/sh
if [ -d /var/lib/docker/plugins/seaweedfs/$1 ]; then
    cat <<EOF >> /etc/haproxy/haproxy.cfg
backend http
    mode http
    server http filer:8888 check
backend grpc
    mode http
    server grpc filer:18888 check proto h2
frontend http
    mode http
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/http.sock
    default_backend http
frontend grpc
    mode http
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/grpc.sock proto h2
    default_backend grpc
EOF
    /usr/local/sbin/haproxy -f /etc/haproxy/haproxy.cfg
fi
echo "/var/lib/docker/plugins/seaweedfs/$1 not found"