#!/bin/sh
cat <<EOF > /etc/nginx/nginx.conf
# Run nginx as a normal console program, not as a daemon
daemon off;

# Log errors to stdout
error_log /dev/stdout info;

events {} # Boilerplate

http {
    # Print the access log to stdout
    access_log /dev/stdout;

    server {
        listen unix:/var/lib/docker/plugins/seaweedfs/$1/http.sock;
        location / {
            proxy_pass http://filer:8888;
        }
    }
    server {
        listen unix:/var/lib/docker/plugins/seaweedfs/$1/grpc.sock http2;
        location / {
            grpc_pass grpc://filer:18888;
        }
    }
}
EOF

/docker-entrypoint.sh "nginx"