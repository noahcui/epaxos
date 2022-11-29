#!/usr/bin/env bash

PID_FILE0=server0.pid
PID_FILE1=server1.pid
PID_FILE2=server2.pid

DIR="testresult"
GROUP="default"
TIME=10
ALG="mo"
KILL=-1
CLIENTS=1
while getopts "a:d:t:k:c:g:" OPTION; 
do
    case "$OPTION" in
    d)
        DIR=$OPTARG
        ;;
    t)
        TIME=$OPTARG
        ;;
    a)
        ALG=$OPTARG
        ;;
    c)
        CLIENTS=$OPTARG
        ;;
    g)
        GROUP=$OPTARG
        ;;
    k)
        KILL=$OPTARG
        ;;
    esac
done

mkdir $DIR/
mkdir $DIR/$GROUP/

export GOPATH=/Users/noahcui/Desktop/UNH/22fall/Research/epaxos


bin/server -port 7070 -exec -dreply -$ALG &
echo $! >> ${PID_FILE0}
bin/server -port 7071 -exec -dreply -$ALG &
echo $! >> ${PID_FILE1}
bin/server -port 7072 -exec -dreply -$ALG &
echo $! >> ${PID_FILE2}
# give it some time to settle down
sleep 5 
bin/client -e -t $TIME -T $CLIENTS -think 1 > $DIR/$GROUP/$CLIENTS-$TIME &

if((KILL>0)); then
sleep $KILL
./stop.sh $PID_FILE0
fi

sleep $TIME
sleep $TIME
./stop.sh $PID_FILE0
./stop.sh $PID_FILE1
./stop.sh $PID_FILE2