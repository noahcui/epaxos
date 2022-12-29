#!/usr/bin/env bash
server1="172.31.29.62"
server2="172.31.16.215"
server3="172.31.25.126"
server4="172.31.31.39"
server5="172.31.27.107"
master="172.31.26.24"
servers=($master $server1 $server2 $server3 $server4 $server5)
PID_FILE0=server0.pid
PID_FILE1=server1.pid
PID_FILE2=server2.pid

DIR="testresult"
GROUP="default"
TIME=10
ALG="mo"
KILL=-1
CLIENTS=1
S=3
while getopts "a:d:t:k:c:g:s:" OPTION; 
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
    s)
        S=$OPTARG
        ;;
    esac
done

mkdir $DIR/
mkdir $DIR/$GROUP/
mkdir $DIR/$GROUP/raw_data
mkdir $DIR/$GROUP/raw_data/useage

export GOPATH=/Users/noahcui/Desktop/UNH/22fall/Research/epaxos

bin/master -N $S &
sleep 1

for ((i=1;i<=$S;i++))
do
    ssh server$i "cd epaxos/bin; ./monitor.py > server$i.csv" &
    ssh server$i "cd epaxos; ./start.sh $master ${servers[$i]} $ALG server.pid" &
done
# ssh server1 "cd epaxos/bin; ./monitor.py > server1.csv" &
# ssh server2 "cd epaxos/bin; ./monitor.py > server2.csv" &
# ssh server3 "cd epaxos/bin; ./monitor.py > server3.csv" &

# ssh server1 "cd epaxos; ./start.sh $master $server1 $ALG server.pid" &
# ssh server2 "cd epaxos; ./start.sh $master $server2 $ALG server.pid" &
# ssh server3 "cd epaxos; ./start.sh $master $server3 $ALG server.pid" &
# bin/server -port 7070 -exec -dreply -$ALG & 
# echo $! >> ${PID_FILE0}
# bin/server -port 7071 -exec -dreply -$ALG &
# echo $! >> ${PID_FILE1}
# bin/server -port 7072 -exec -dreply -$ALG &
# echo $! >> ${PID_FILE2}
# give it some time to settle down
sleep 5 
bin/client -e -t $TIME -T $CLIENTS -think 1 -path $DIR/$GROUP/raw_data/$CLIENTS-$TIME > $DIR/$GROUP/$CLIENTS-$TIME &

if((KILL>0)); then
sleep $KILL
ssh server1 "cd epaxos; ./stop.sh server.pid;" &
fi
mkdir $DIR/$GROUP/raw_data/useage/$CLIENTS-$TIME
# sleep $TIME
sleep $TIME
sleep 5
for ((i=1;i<=$S;i++))
do
    scp server$i:epaxos/server$i.csv $DIR/$GROUP/raw_data/useage/$CLIENTS-$TIME/
    ssh server$i "cd epaxos; ./stop.sh server.pid;" &
done

# ssh server1 "cd epaxos; ./stop.sh server.pid;" &
# ssh server2 "cd epaxos; ./stop.sh server.pid;" &
# ssh server3 "cd epaxos; ./stop.sh server.pid;" &