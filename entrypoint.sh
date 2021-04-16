#!/bin/sh
HAPROXY_PATH=${HAPROXY_PATH:-"/usr/sbin/haproxy"}
HAPROXY_CONF_PATH=${HAPROXY_CONF_PATH:-"/etc/haproxy/haproxy.cfg"}
DATAPLANEAPI_PATH=${DATAPLANEAPI_PATH:="/usr/sbin/dataplaneapi"}
DATAPLANEAPI_HOST=${DATAPLANEAPI_HOST:="localhost"}
DATAPLANEAPI_PORT=${DATAPLANEAPI_PORT:="5555"}
DATAPLANEAPI_USERNAME=${DATAPLANEAPI_USERNAME:-"dataplaneapi"}
DATAPLANEAPI_PASSWORD=${DATAPLANEAPI_PASSWORD:-"haproxyapi"}
cat <<EOF >> /etc/haproxy/haproxy.cfg
userlist haproxy-dataplaneapi
    user $DATAPLANEAPI_USERNAME password $(mkpasswd -m sha-256 $DATAPLANEAPI_PASSWORD)
program api
    command $DATAPLANEAPI_PATH --host $DATAPLANEAPI_HOST --port $DATAPLANEAPI_PORT --haproxy-bin $HAPROXY_PATH --config-file $HAPROXY_CONF_PATH --reload-cmd "systemctl reload haproxy" --reload-delay 5 --userlist haproxy-dataplaneapi
    no option start-on-reload
EOF
/usr/bin/supervisord -c /etc/supervisord.conf