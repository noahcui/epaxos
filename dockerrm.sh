for ((i=1; i<=$1; i++))
do
docker rm -f -v s$i
done
docker rm -f -v sb
docker image rm $2
docker network rm epaxosNet
