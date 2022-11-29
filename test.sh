for ((i=8;i<=80;i+=8))
do
./driver.sh -d testresult -t 5 -g batching -c $i -a mo

sleep 120
done

sleep 240

for ((i=8;i<=64;i+=8))
do
./driver.sh -d testresult -t 5 -g nobatching -c $i -a m

sleep 120
done