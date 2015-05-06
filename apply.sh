#!/bin/bash

source ${CATTLE_HOME:-/var/lib/cattle}/common/scripts.sh

trap "touch $CATTLE_HOME/.pyagent-stamp" exit

cd $(dirname $0)

mkdir -p ${CATTLE_HOME}/bin

PID=$(pidof host-api || true)
if [ -n "${PID}" ]; then
    kill $PID
    sleep 1
fi

cp bin/host-api bin/net-util.sh ${CATTLE_HOME}/bin

chmod +x ${CATTLE_HOME}/bin/host-api
chmod +x ${CATTLE_HOME}/bin/net-util.sh

