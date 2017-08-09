#!/usr/bin/env bash
# ******************************************************
# DESC    : PalmChat redis-cluster cluster template script
# AUTHOR  : Alex Stocks
# VERSION : 1.0
# LICENCE : LGPL V3
# EMAIL   : alexstocks@foxmail.com
# MOD     : 2016-04-06 16:41
# FILE    : cluster.sh
# ******************************************************

meta_db_mem_size='50m'

gen() {
	cd script
	for ((idx = 0; idx < $1; idx ++))
	do
		python template.py master $idx $2
		python template.py slave $idx $2
	done
	python template.py meta-master $meta_db_mem_size
	python template.py meta-slave $meta_db_mem_size
	for ((idx = 0; idx < 3; idx ++))
	do
		python template.py sentinel $idx
	done
	rm -rf ./cluster
	mkdir -p ./cluster
	mv master* slave* sentinel* meta* ./cluster/
	mkdir -p ./cluster/script/
	cp cluster_notify.sh ./cluster/script/
	cp cluster_notify.sh /tmp/
	cp cluster_load.sh cluster/
	cp -rf ../client/ cluster/
	rm -rf ../cluster/
	mv cluster ../
}

start() {
	local_ip=0.0.0.0
	# local_ip=10.136.40.61
	master_port=4000
	slave_port=14000
	instance_set=""
	for ((idx = 0; idx < $1; idx ++))
	do
		[[ -d master$idx ]] && cd master$idx && sh master-load.sh start $local_ip $master_port && cd ..
		sleep 2
		[[ -d slave$idx ]] && cd slave$idx  && sh slave-load.sh  start $local_ip $slave_port $local_ip $master_port && cd ..
		instance_set="${instance_set}@cache${idx}:$local_ip:${master_port}"
		((master_port ++))
		((slave_port ++))
	done
	instance_set=${instance_set:1}
	cp script/cluster_notify.sh /tmp/
	sentinel_port=26380
	for ((idx = 0; idx < 3; idx ++))
	do
		[[ -d sentinel$idx ]] && cd sentinel$idx && sh sentinel-load.sh start $local_ip $sentinel_port /tmp/cluster_notify.sh $instance_set && cd ..
		((sentinel_port ++))
	done
}

stop() {
	for ((idx = 0; idx < $1; idx ++))
	do
		[[ -d master$idx ]] && cd master$idx && sh master-load.sh stop && cd ..
		sleep 2
		[[ -d slave$idx ]] && cd slave$idx  && sh slave-load.sh  stop && cd ..
	done
	sleep 2
	for ((idx = 0; idx < 3; idx ++))
	do
		[[ -d sentinel$idx ]] && cd sentinel$idx && sh sentinel-load.sh stop && cd ..
	done
}

clean() {
	for ((idx = 0; idx < $1; idx ++))
	do
		[[ -d master$idx ]] && cd master$idx && sh master-load.sh clean && cd ..
		[[ -d slave$idx ]] && cd slave$idx  && sh slave-load.sh  clean && cd ..
	done
	sleep 2
	for ((idx = 0; idx < 3; idx ++))
	do
		[[ -d sentinel$idx ]] && cd sentinel$idx && sh sentinel-load.sh clean && cd ..
	done
}

case C"$1" in
	Cstart)
		if [ "-$2" = "-" ]; then
			echo "Please Input: $0 start instance-number"
		else
			start $2
			echo "start Done!"
		fi
		;;
	Cstop)
		if [ "-$2" = "-" ]; then
			echo "Please Input: $0 stop instance-number"
		else
			stop $2
			echo "stop Done!"
		fi
		;;
	Cgen)
		if [ "-$3" = "-" ]; then
			echo "Please Input: $0 gen instance-number memory-size"
		else
			gen $2 $3
			echo "gen Done!"
		fi
		;;
	Cclean)
		if [ "-$2" = "-" ]; then
			echo "Please Input: $0 clean instance-number"
		else
			stop $2
			clean $2
			echo "clean Done!"
		fi
		;;
	C*)
		echo "Usage: $0 {start|stop|clean|gen}"
		;;
esac

