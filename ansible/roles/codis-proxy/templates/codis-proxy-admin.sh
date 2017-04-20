#!/usr/bin/env bash

CODIS_ADMIN="${BASH_SOURCE-$0}"
CODIS_ADMIN="$(dirname "${CODIS_ADMIN}")"
CODIS_ADMIN_DIR="$(cd "${CODIS_ADMIN}"; pwd)"

CODIS_BIN_DIR=$CODIS_ADMIN_DIR/../bin
CODIS_LOG_DIR=$CODIS_ADMIN_DIR/../log
CODIS_CONF_DIR=$CODIS_ADMIN_DIR/../config

CODIS_PROXY_BIN=$CODIS_BIN_DIR/codis-proxy
CODIS_PROXY_PID_FILE=$CODIS_BIN_DIR/codis-proxy.pid

CODIS_PROXY_LOG_FILE=$CODIS_LOG_DIR/codis-proxy.log
CODIS_PROXY_DAEMON_FILE=$CODIS_LOG_DIR/codis-proxy.out

CODIS_PROXY_CONF_FILE=$CODIS_CONF_DIR/proxy.toml

CODIS_DASHBOARD_ADDR="{{ dashboard_addr }}"

echo $CODIS_PROXY_CONF_FILE

if [ ! -d $CODIS_LOG_DIR ]; then
    mkdir -p $CODIS_LOG_DIR
fi


case $1 in
start)
    echo  "starting codis-proxy ... "
    if [ -f "$CODIS_PROXY_PID_FILE" ]; then
      if kill -0 `cat "$CODIS_PROXY_PID_FILE"` > /dev/null 2>&1; then
         echo $command already running as process `cat "$CODIS_PROXY_PID_FILE"`.
         exit 0
      fi
    fi
    nohup "$CODIS_PROXY_BIN" "--config=${CODIS_PROXY_CONF_FILE}" "--dashboard=${CODIS_DASHBOARD_ADDR}" \
    "--log=$CODIS_PROXY_LOG_FILE" "--log-level=INFO" "--ncpu=4" "--pidfile=$CODIS_PROXY_PID_FILE" > "$CODIS_PROXY_DAEMON_FILE" 2>&1 < /dev/null &
    ;;
start-foreground)
    $CODIS_PROXY_BIN "--config=${CODIS_PROXY_CONF_FILE}" "--dashboard=${CODIS_DASHBOARD_ADDR}" \
    "--log-level=DEBUG" "--pidfile=$CODIS_PROXY_PID_FILE"
    ;;
stop)
    echo "stopping codis-proxy ... "
    if [ ! -f "$CODIS_PROXY_PID_FILE" ]
    then
      echo "no codis-proxy to stop (could not find file $CODIS_PROXY_PID_FILE)"
    else
      kill -2 $(cat "$CODIS_PROXY_PID_FILE")
      echo STOPPED
    fi
    exit 0
    ;;
stop-forced)
    echo "stopping codis-proxy ... "
    if [ ! -f "$CODIS_PROXY_PID_FILE" ]
    then
      echo "no codis-proxy to stop (could not find file $CODIS_PROXY_PID_FILE)"
    else
      kill -9 $(cat "$CODIS_PROXY_PID_FILE")
      rm "$CODIS_PROXY_PID_FILE"
      echo STOPPED
    fi
    exit 0
    ;;
restart)
    shift
    "$0" stop
    sleep 1
    "$0" start
    ;;
*)
    echo "Usage: $0 {start|start-foreground|stop|stop-forced|restart}" >&2

esac
