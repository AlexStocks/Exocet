redis_cluster
===============

redis cluster  with sentinel

# redis-server
---
The redis binary (redis_cluster/script/template/redis-server) version is v3.0.2 used as v2.8 instance. U can upgrade it by compiling the redis codes from its [offical github repo](https://github.com/antirez/redis).

# how to start the cluster on one host
---
* edit script/cluster_load.sh and change the address(ip&port) of meta & master & slave & sentinel   

		If you wanna more than 2 redis instances, please add instance2's address{master2_ip, master2_port, slave2_ip, slave2_port} and so on.
		
* gen the cluster by cluster.sh: sh cluster.sh gen 2 100m

		the parameter 2 means cluster.sh should generate 2 redis instances and the parameter 100m means the instance's max memory size is 100MB.
		
* and then you gotta the cluster directory, step into this dir and start the cluster as follows

		sh cluster_load.sh start meta
		sh cluster_load.sh start master
		sh cluster_load.sh start slave
		sh cluster_load.sh start sentinel
		
* if you wanna shutdown the cluster, do it as follows

		sh cluster_load.sh stop sentinel
		sh cluster_load.sh stop slave
		sh cluster_load.sh stop master
		sh cluster_load.sh stop meta