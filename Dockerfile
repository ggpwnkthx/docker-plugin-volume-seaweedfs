FROM golang:1-alpine as builder
RUN set -ex \
    && apk add --no-cache --virtual .build-deps \
    gcc libc-dev git
COPY ./src /src
WORKDIR /src
RUN go get github.com/docker/go-plugins-helpers/volume
RUN go get github.com/chrislusf/seaweedfs
RUN go get github.com/sirupsen/logrus
RUN go build -o /bin/docker-plugin-volume

FROM alpine:3
#COPY --from=chrislusf/seaweedfs /usr/bin/weed /usr/bin/
COPY --from=builder /bin/docker-plugin-volume /bin/docker-plugin-volume
CMD ["/bin/docker-plugin-volume"]