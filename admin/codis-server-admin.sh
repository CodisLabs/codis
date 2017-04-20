#!/usr/bin/env bash

CODIS_ADMIN="${BASH_SOURCE-$0}"
CODIS_ADMIN="$(dirname "${CODIS_ADMIN}")"
CODIS_ADMIN_DIR="$(cd "${CODIS_ADMIN}"; pwd)"

CODIS_BIN_DIR=$CODIS_ADMIN_DIR/../bin
CODIS_LOG_DIR=$CODIS_ADMIN_DIR/../log
CODIS_CONF_DIR=$CODIS_ADMIN_DIR/../config

CODIS_SERVER_BIN=$CODIS_BIN_DIR/codis-server
CODIS_SERVER_PID_FILE=/tmp/redis_6379.pid

CODIS_SERVER_LOG_FILE=/tmp/redis_6379.log
CODIS_SERVER_DAEMON_FILE=$CODIS_LOG_DIR/codis-server.out

CODIS_SERVER_CONF_FILE=$CODIS_CONF_DIR/redis.conf

echo $CODIS_SERVER_CONF_FILE

if [ ! -d $CODIS_LOG_DIR ]; then
    mkdir -p $CODIS_LOG_DIR
fi


case $1 in
start)
    echo  "starting codis-server ... "
    if [ -f "$CODIS_SERVER_PID_FILE" ]; then
      if kill -0 `cat "$CODIS_SERVER_PID_FILE"` > /dev/null 2>&1; then
         echo $command already running as process `cat "$CODIS_SERVER_PID_FILE"`.
         exit 0
      fi
    fi
    nohup "$CODIS_SERVER_BIN" "${CODIS_SERVER_CONF_FILE}" > "$CODIS_SERVER_DAEMON_FILE" 2>&1 < /dev/null &
    ;;
stop)
    echo "stopping codis-server ... "
    if [ ! -f "$CODIS_SERVER_PID_FILE" ]
    then
      echo "no codis-server to stop (could not find file $CODIS_SERVER_PID_FILE)"
    else
      kill -2 $(cat "$CODIS_SERVER_PID_FILE")
      echo STOPPED
    fi
    exit 0
    ;;
stop-forced)
    echo "stopping codis-server ... "
    if [ ! -f "$CODIS_SERVER_PID_FILE" ]
    then
      echo "no codis-server to stop (could not find file $CODIS_SERVER_PID_FILE)"
    else
      kill -9 $(cat "$CODIS_SERVER_PID_FILE")
      rm "$CODIS_SERVER_PID_FILE"
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
    echo "Usage: $0 {start|stop|stop-forced|restart}" >&2

esac
