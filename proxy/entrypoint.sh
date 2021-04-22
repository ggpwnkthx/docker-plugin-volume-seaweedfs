#!/bin/sh
if [ -d /var/lib/docker/plugins/seaweedfs/$1 ]; then
    cat <<EOF >> /etc/haproxy/haproxy.cfg
backend http
    mode http
    server http filer:8888
backend grpc
    mode http
    server grpc filer:18888 ssl verify none apln h2
frontend http
    mode http
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/http.sock
    default_backend http
frontend grpc
    mode http
    bind unix@/var/lib/docker/plugins/seaweedfs/$1/grpc.sock apln h2
    default_backend grpc
EOF
    /usr/bin/supervisord -c /etc/supervisord.conf
fi
echo "/var/lib/docker/plugins/seaweedfs/$1 not found"