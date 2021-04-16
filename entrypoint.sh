#!/bin/sh
DATAPLANEAPI_USERNAME=${DATAPLANEAPI_USERNAME:-"dataplaneapi"}
DATAPLANEAPI_PASSWORD=${DATAPLANEAPI_PASSWORD:-"haproxyapi"}
cat <<EOF >> /etc/haproxy/haproxy.cfg
userlist haproxy-dataplaneapi
    user $DATAPLANEAPI_USERNAME password $(mkpasswd -m sha-256 $DATAPLANEAPI_PASSWORD)
EOF
/usr/bin/supervisord -c /etc/supervisord.conf
