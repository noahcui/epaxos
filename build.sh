export GOPATH=/home/ubuntu/epaxos/
git pull
# rm -rf bin
go install server
go install client
go install master
bin/master
