# export GOPATH=/home/ubuntu/epaxos/
# git pull
# rm -rf bin
cd bin
go build ../src/server
go build ../src/client
go build ../src/master
cd ..
# bin/master
