#!/bin/bash
#serverArray=("server1" "server2" "server3" "server4" "server5")
serverArray=("server1" "server2" "server3")
for server in ${serverArray[@]}; 
do
    ssh $server "sudo reboot"
done

# sudo reboot
