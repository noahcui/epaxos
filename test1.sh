#!/usr/bin/env bash
for ((i=100;i<=128;i+=4))
do
	echo "mo $i"
	sleep 5
	./AWSreboot.sh
	sleep 30
	./driver.sh -d testresult -t 60 -g batching -c $i -a mo
	
	echo "mo $i finish"
done