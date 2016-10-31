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

cp bin/host-api ${CATTLE_HOME}/bin

chmod +x ${CATTLE_HOME}/bin/host-api
