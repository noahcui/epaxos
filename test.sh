#!/usr/bin/env bash

PID_FILE0=server0.pid
PID_FILE1=server1.pid
PID_FILE2=server2.pid

# for i in 0 1 2
# do
# ./stop.sh ${PID_FILE$i}
# done

mkdir testresult
mkdir testresult/$1
mkdir testresult/$1/$2

export GOPATH=/Users/noahcui/Desktop/UNH/22fall/Research/epaxos


bin/server -port 7070 -exec -mo  &
echo $! >> ${PID_FILE0}
bin/server -port 7071 -exec -mo  &
echo $! >> ${PID_FILE1}
bin/server -port 7072 -exec -mo  &
echo $! >> ${PID_FILE2}
# give it some time to settle down
sleep 5 
bin/client -e -t 20 -T $2 -think 1 > testresult/$1/$2/clientresult &

sleep 5
# ./stop.sh $PID_FILE0

sleep 30
./stop.sh $PID_FILE0
./stop.sh $PID_FILE1
./stop.sh $PID_FILE2