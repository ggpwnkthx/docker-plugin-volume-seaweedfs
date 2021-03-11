FROM debian:stable-slim
COPY --from=chrislusf/seaweedfs /usr/bin/weed /usr/bin/
COPY docker-plugin-volume-seaweedfs /bin/docker-plugin-volume-seaweedfs
RUN chmod +x /bin/docker-plugin-volume-seaweedfs
