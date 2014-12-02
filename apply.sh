#!/bin/bash

source ${CATTLE_HOME:-/var/lib/cattle}/common/scripts.sh

cd $(dirname $0)

mkdir -p ${CATTLE_HOME}/bin

cp bin/host-api ${CATTLE_HOME}/bin

chmod +x ${CATTLE_HOME}/bin/host-api

STAMP=$CATTLE_HOME/.pyagent-stamp
