#!/usr/bin/env bash
rm -rf testresult
for ((i=4;i<=128;i+=4))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g batching -c $i -a mo
	
	echo "mo $i finish"
done

sleep 240

for ((i=4;i<=80;i+=4))
do
	echo "m $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g nobatching -c $i -a m
	echo "m $i finish"
done

