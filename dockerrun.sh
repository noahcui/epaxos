docker build -t epaxos:test -f Dockerfile .
docker network create epaxosNet
for ((i=1; i<=$1; i++))
do
    docker run -d -d -p $((7070 + i)):$((7070 + i))  -i --name s$i $2 bash
    docker network connect epaxosNet s$i
    docker update --cpus 1 s$i
    docker update --memory 2g --memory-swap 4g s$i
done
docker run -d -p 8080:8080 -i --name sb $2  bash
docker network connect epaxosNet sb
docker update --cpus 2 sb
# docker update --cpus 0.2 s3
docker update --memory 2g --memory-swap 4g sb
# ./server -config config.json -log_level info -id 1.1
# ./benchmark -zone 1 -bconfig benchmark.json -log_level info -config config.json
# ./server -config configrb.json -log_level info -id 1.1
# ./benchmark -zone 1 -bconfig benchmark.json -log_level info -config configrb.json
# for ((i=1; i<=$1; i++))
# do      

    # docker ps -q | xargs docker inspect -f '{{ .Name }}: {{ .NetworkSettings.IPAddress }}' | sed 's/^\\///'

    # echo s$i
    # docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' s$i
# done

# echo sb
# docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sb
