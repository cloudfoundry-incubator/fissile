#!/bin/bash

RUN_DIR=/var/vcap/sys/run/ntpd
# PID_FILE is created by named, not by this script
PID_FILE=${RUN_DIR}/ntpd.pid
# ntpd is on stemcell by default

case $1 in

  start)
    # we must stop and disable chronyd on CentOS; it contends for NTP port 123
    systemctl disable chronyd; systemctl stop chronyd
    # kick off ntpd
    mkdir -p $RUN_DIR
    chown -R vcap:vcap $RUN_DIR
    exec /var/vcap/packages/ntp-4.2.8p2/bin/ntpd -u vcap:vcap -p $PID_FILE -c /var/vcap/jobs/ntpd/etc/ntp.conf
    ;;

  stop)
    PID=$(cat $PID_FILE)
    kill -TERM $PID
    sleep 1
    kill -KILL $PID

    rm -rf $PID_FILE
    ;;

  *)
    echo "Usage: ctl {start|stop}" ;;

esac
