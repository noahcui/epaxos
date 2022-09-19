cd ..
while getopts "p:c:n:" OPTION; 
do
    case "$OPTION" in
    p)
        PORT=$OPTARG
        P=$((PORT+1000))
        ;;
    c)
        CMD=$OPTARG
        ;;
    n)
        NAME=$OPTARG
        ;;
    esac
done
docker run -e PORT=$PORT -e P=$P -e CMD="$CMD" --name $NAME -dp $PORT:$PORT -dp $P:$P epaxos:latest
echo $PORT $P