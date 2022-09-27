export GOPATH=/home/ubuntu/epaxos/

git pull
rm -rf bin
go build server
go build client
go build master
bin/master
