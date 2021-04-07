FROM golang:1-alpine as builder
RUN apk add --no-cache --virtual .build-deps gcc libc-dev git
COPY ./src /go/src/driver
WORKDIR /go/src/driver
RUN go mod tidy
RUN go mod download
RUN go build -o /bin/docker-plugin-volume

FROM alpine:3
RUN apk add --no-cache fuse
RUN mkdir -p /var/run/docker/plugins/seaweedfs
COPY --from=ggpwnkthx/seaweedfs:latest /usr/bin/weed /usr/bin/weed
COPY --from=builder /bin/docker-plugin-volume /bin/docker-plugin-volume
CMD ["/bin/docker-plugin-volume"]