redis_cluster
===============

redis cluster  with sentinel


# how to start the cluster on one host

* gen the cluster by cluster.sh: sh cluster.sh gen 2 100m

		- the parameter 2 means cluster.sh should generate 2 redis instances
		- the parameter 100m means the instance's max memory size is 100MB.

* and then you gotta the cluster directory, step into this dir and start the cluster as follows

		sh cluster.sh start 2

* if you wanna shutdown the cluster, do it as follows

		sh cluster.sh stop 2
		
# Attention
* In production environment, place the 3 sentinels on 3 different hosts.
* In production environment, place redis instance on host where you should not place sentinel. Otherwise metaserver will get the redis host address "127.0.0.1" which the metaserver or the proxy can not connect to it. 