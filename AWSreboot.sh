#!/bin/bash
serverArray=("server1" "server2" "server3" "benchmarker")

for server in ${serverArray[@]}; 
do
    ssh $server "sudo reboot"
done

# sudo reboot
