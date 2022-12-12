server -port 7070 -exec -dreply -maddr $1 -addr $2 -$3 &
echo $! >> ${PID_FILE0}