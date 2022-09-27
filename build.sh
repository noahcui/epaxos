export GOPATH=/home/ubuntu/epaxos/

go build server
go build client
go build master

scp -r benchmarker:epaxos/bin server1:epaxos/
scp -r benchmarker:epaxos/bin server2:epaxos/
scp -r benchmarker:epaxos/bin server3:epaxos/