#!/usr/bin/env bash
#rm -rf testresult

for ((i=10;i<=200;i+=5))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3batching3ms_t10 -c $i -a mo -s 3 -i 5
	
	echo "mo $i finish"
done

sleep 240

for ((i=1;i<=20;i+=2))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3nobatching3ms_t10 -c $i -a m -s 3 -i 5
	
	echo "mo $i finish"
done

for ((i=10;i<=200;i+=5))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3batching3ms_t10 -c $i -a mo -s 3 -i 10
	
	echo "mo $i finish"
done

sleep 240

for ((i=2;i<=40;i+=2))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3nobatching3ms_t10 -c $i -a m -s 3 -i 10
	
	echo "mo $i finish"
done



./AWSshutdown
sudo shutdown
