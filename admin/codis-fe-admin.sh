#!/usr/bin/env bash

CODIS_ADMIN="${BASH_SOURCE-$0}"
CODIS_ADMIN="$(dirname "${CODIS_ADMIN}")"
CODIS_ADMIN_DIR="$(cd "${CODIS_ADMIN}"; pwd)"

CODIS_BIN_DIR=$CODIS_ADMIN_DIR/../bin
CODIS_LOG_DIR=$CODIS_ADMIN_DIR/../log
CODIS_CONF_DIR=$CODIS_ADMIN_DIR/../config

CODIS_FE_BIN=$CODIS_BIN_DIR/codis-fe
CODIS_FE_PID_FILE=$CODIS_BIN_DIR/codis-fe.pid
CODIS_FE_ASSETS_DIR=$CODIS_BIN_DIR/assets

CODIS_FE_LOG_FILE=$CODIS_LOG_DIR/codis-fe.log
CODIS_FE_DAEMON_FILE=$CODIS_LOG_DIR/codis-fe.out

COORDINATOR_NAME="filesystem"
COORDINATOR_ADDR="/tmp/codis"
CODIS_FE_ADDR="0.0.0.0:9090"

echo $CODIS_FE_CONF_FILE

if [ ! -d $CODIS_LOG_DIR ]; then
    mkdir -p $CODIS_LOG_DIR
fi


case $1 in
start)
    echo  "starting codis-fe ... "
    if [ -f "$CODIS_FE_PID_FILE" ]; then
      if kill -0 `cat "$CODIS_FE_PID_FILE"` > /dev/null 2>&1; then
         echo $command already running as process `cat "$CODIS_FE_PID_FILE"`.
         exit 0
      fi
    fi
    nohup "$CODIS_FE_BIN" "--assets-dir=${CODIS_FE_ASSETS_DIR}" "--$COORDINATOR_NAME=$COORDINATOR_ADDR" \
    "--log=$CODIS_FE_LOG_FILE" "--pidfile=$CODIS_FE_PID_FILE" "--log-level=INFO" "--listen=$CODIS_FE_ADDR" > "$CODIS_FE_DAEMON_FILE" 2>&1 < /dev/null &
    ;;
start-foreground)
    $CODIS_FE_BIN "--assets-dir=${CODIS_FE_ASSETS_DIR}" "--$COORDINATOR_NAME=$COORDINATOR_ADDR" \
    "--log-level=DEBUG" "--listen=$CODIS_FE_ADDR"
    ;;
stop)
    echo "stopping codis-fe ... "
    if [ ! -f "$CODIS_FE_PID_FILE" ]
    then
      echo "no codis-fe to stop (could not find file $CODIS_FE_PID_FILE)"
    else
      kill -9 $(cat "$CODIS_FE_PID_FILE")
      rm $CODIS_FE_PID_FILE
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
    echo "Usage: $0 {start|start-foreground|stop|restart}" >&2

esac
