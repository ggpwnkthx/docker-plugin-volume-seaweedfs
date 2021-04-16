# Compile plugin driver in a different container
FROM golang:1-alpine as builder
RUN apk update && apk add --no-cache --virtual .build-deps gcc libc-dev git
COPY ./src /go/src/driver
WORKDIR /go/src/driver
RUN go mod tidy
RUN go mod download
RUN go build -o /bin/docker-plugin-volume

# Build runtime container image
FROM alpine:3
RUN apk update && apk add --no-cache fuse supervisor haproxy rsyslog
COPY ./supervisord.conf /etc/supervisord.conf
RUN mkdir -p /var/run/docker/plugins/seaweedfs
# Get SeaweedFS binary
COPY --from=ggpwnkthx/seaweedfs:latest /usr/bin/weed /usr/bin/weed
# Get plugin driver binary compliled earlier
COPY --from=builder /bin/docker-plugin-volume /bin/docker-plugin-volume
# Get HAProxy binary and etc
COPY --from=ggpwnkthx/docker-plugin-volume-seaweedfs-filer-proxy /usr/local/sbin/dataplaneapi /usr/sbin/dataplaneapi
COPY --from=ggpwnkthx/docker-plugin-volume-seaweedfs-filer-proxy /usr/local/etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg
RUN mkdir -p /tmp/haproxy/transactions

COPY ./entrypoint.sh /bin/entrypoint.sh
RUN chmod +x /bin/entrypoint.sh
CMD [ "/bin/entrypoint.sh" ]