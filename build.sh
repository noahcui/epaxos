export GOPATH=/home/ubuntu/epaxos/

git pull
go build server
go build client
go build master
bin/master
