#!/usr/bin/env bash
#rm -rf testresult
for ((i=5;i<=200;i+=5))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3batching3ms -c $i -a mo -s 3
	
	echo "mo $i finish"
done

sleep 240

for ((i=2;i<=40;i+=1))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3nobatching3 -c $i -a m -s 3
	
	echo "mo $i finish"
done


