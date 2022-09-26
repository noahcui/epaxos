cd ..
go install master
go install server
go install client
ip=10.21.108.31
bin/master &
pidm=$!
bin/server -exec -port 7070 -maddr $ip -addr $ip -m &
pids0=$!
bin/server -exec -port 7071 -maddr $ip -addr $ip -m &
pids1=$!
bin/server -exec -port 7072 -maddr $ip -addr $ip -m &
pids2=$!
sleep 3
echo $pids2 $pids1 $pids0 $pidm

# kill $pids0
sleep 3
bin/client -maddr $ip

kill $pidm
kill $pids1
kill $pids2
kill $pids0