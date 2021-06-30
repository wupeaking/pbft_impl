#!/bin/bash
#获取原进程id
old_pid=`ps -ef|grep "counch.x"|grep -v grep|awk '{print $2}'`

#重启
if [ "$old_pid" != '' ];then
    echo "begin to stop $old_pid"
    kill -9 $old_pid
    echo "$old_pid stoped, "
    sleep 3
fi

chmod +x counch.x

nohup ./counch.x \
>> counch.log 2>&1 &