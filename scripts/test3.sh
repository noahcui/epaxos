./build.sh  
./kill.sh 3
addr="10.21.137.206"
./run.sh -c "bin/master -port 7087" -p 7087 -n master
sleep 3
./run.sh -c "bin/server -port 7070 -maddr $addr -addr $addr" -p 7070 -n server0
./run.sh -c "bin/server -port 7071 -maddr $addr -addr $addr" -p 7071 -n server1
./run.sh -c "bin/server -port 7072 -maddr $addr -addr $addr" -p 7072 -n server2