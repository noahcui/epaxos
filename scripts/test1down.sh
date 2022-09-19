cd ..
go install master
go install server
go install client
bin/master &
pidm=$!
bin/server -exec -port 7070 -maddr 10.21.137.206 -addr 10.21.137.206 -m &
pids0=$!
bin/server -exec -port 7071 -maddr 10.21.137.206 -addr 10.21.137.206 -m &
pids1=$!
bin/server -exec -port 7072 -maddr 10.21.137.206 -addr 10.21.137.206 -m &
pids2=$!
sleep 3
echo $pids2 $pids1 $pids0 $pidm

kill $pids0
sleep 3
bin/client -maddr 10.21.137.206 -q 500005 -e

kill $pidm
kill $pids1
kill $pids2
# kill $pids0