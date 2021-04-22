FROM haproxy:alpine
RUN apk update && apk add --no-cache curl tar supervisor
COPY ./haproxy.cfg /etc/haproxy/haproxy.cfg
COPY ./supervisord.conf /etc/supervisord.conf
# Get HAProxy DataPlaneAPI binary
RUN ARCH="$(apk --print-arch)"; \
    case "$ARCH" in \
        armhf) ASSET="Linux_arm.tar.gz" ;; \
        arm64) ASSET="Linux_arm64.tar.gz" ;; \
        x86) ASSET="Linux_i386.tar.gz" ;; \
        x86_64) ASSET="Linux_x86_64.tar.gz" ;; \
    esac; \
    curl -L $(curl -s https://api.github.com/repos/haproxytech/dataplaneapi/releases/latest | grep "browser_download_url.*$ASSET" | cut -d '"' -f 4) -o /tmp/$ASSET; \
    cd /tmp; \
    tar -zxvf $ASSET
RUN mv /tmp/build/dataplaneapi /usr/sbin/dataplaneapi
# Clean up
RUN apk del curl tar
RUN rm -r /tmp/*
# Set entrypoint
COPY ./entrypoint.sh /bin/entrypoint.sh
RUN chmod +x /bin/entrypoint.sh
ENTRYPOINT [ "/bin/entrypoint.sh" ]