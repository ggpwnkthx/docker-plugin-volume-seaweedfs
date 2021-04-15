#!/bin/sh
DATAPLANEAPI_USERNAME=${DATAPLANEAPI_USERNAME:-"dataplaneapi"}
DATAPLANEAPI_PASSWORD=${DATAPLANEAPI_PASSWORD:-"haproxyapi"}
cat <<EOF >> /usr/local/etc/haproxy/haproxy.cfg
userlist haproxy-dataplaneapi
    user $DATAPLANEAPI_USERNAME password $(mkpasswd -m sha-256 $DATAPLANEAPI_PASSWORD)
program api
    command /usr/local/sbin/dataplaneapi --host 0.0.0.0 --port 5555 --haproxy-bin /usr/local/sbin/haproxy --config-file /usr/local/etc/haproxy/haproxy.cfg --reload-cmd "systemctl reload haproxy" --reload-delay 5 --userlist haproxy-dataplaneapi
    no option start-on-reload
EOF
/usr/bin/supervisord -c /etc/supervisord.conf