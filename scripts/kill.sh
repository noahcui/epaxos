docker kill master
docker rm master
for ((i = 0; i < $1; i++)) do
    docker kill server$i
    docker rm server$i
done