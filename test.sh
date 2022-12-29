#!/usr/bin/env bash
#rm -rf testresult
for ((i=5;i<=200;i+=5))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 3batching -c $i -a mo -s 3
	
	echo "mo $i finish"
done

sleep 240

for ((i=5;i<=200;i+=5))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g 5batching -c $i -a mo -s 5
	
	echo "mo $i finish"
done