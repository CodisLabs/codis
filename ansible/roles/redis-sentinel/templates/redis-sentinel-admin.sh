#!/usr/bin/env bash

CODIS_ADMIN="${BASH_SOURCE-$0}"
CODIS_ADMIN="$(dirname "${CODIS_ADMIN}")"
CODIS_ADMIN_DIR="$(cd "${CODIS_ADMIN}"; pwd)"

CODIS_BIN_DIR=$CODIS_ADMIN_DIR/../bin
CODIS_LOG_DIR=$CODIS_ADMIN_DIR/../log
CODIS_CONF_DIR=$CODIS_ADMIN_DIR/../config

REDIS_SENTINEL_BIN=$CODIS_BIN_DIR/redis-sentinel
REDIS_SENTINEL_PID_FILE={{ redis_sentinel_workdir }}/sentinel_{{ redis_sentinel_port }}.pid

REDIS_SENTINEL_LOG_FILE={{ redis_sentinel_workdir }}/sentinel_{{ redis_sentinel_port }}.log
REDIS_SENTINEL_DAEMON_FILE=$CODIS_LOG_DIR/redis-sentinel.out

REDIS_SENTINEL_CONF_FILE=$CODIS_CONF_DIR/sentinel.conf

echo $REDIS_SENTINEL_CONF_FILE

if [ ! -d $CODIS_LOG_DIR ]; then
    mkdir -p $CODIS_LOG_DIR
fi


case $1 in
start)
    echo  "starting redis-sentinel ... "
    if [ -f "$REDIS_SENTINEL_PID_FILE" ]; then
      if kill -0 `cat "$REDIS_SENTINEL_PID_FILE"` > /dev/null 2>&1; then
         echo $command already running as process `cat "$REDIS_SENTINEL_PID_FILE"`.
         exit 0
      fi
    fi
    nohup "$REDIS_SENTINEL_BIN" "${REDIS_SENTINEL_CONF_FILE}" > "$REDIS_SENTINEL_DAEMON_FILE" 2>&1 < /dev/null &
    ;;
stop)
    echo "stopping redis-sentinel ... "
    if [ ! -f "$REDIS_SENTINEL_PID_FILE" ]
    then
      echo "no redis-sentinel to stop (could not find file $REDIS_SENTINEL_PID_FILE)"
    else
      kill -2 $(cat "$REDIS_SENTINEL_PID_FILE")
      echo STOPPED
    fi
    exit 0
    ;;
stop-forced)
    echo "stopping redis-sentinel ... "
    if [ ! -f "$REDIS_SENTINEL_PID_FILE" ]
    then
      echo "no redis-sentinel to stop (could not find file $REDIS_SENTINEL_PID_FILE)"
    else
      kill -9 $(cat "$REDIS_SENTINEL_PID_FILE")
      rm "$REDIS_SENTINEL_PID_FILE"
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
