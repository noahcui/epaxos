#!/usr/bin/env bash
#rm -rf testresult
for ((i=1;i<=160;i+=2))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g batching -c $i -a mo
	
	echo "mo $i finish"
done

sleep 240

for ((i=1;i<=80;i+=1))
do
	echo "m $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g nobatching -c $i -a m
	echo "m $i finish"
done

