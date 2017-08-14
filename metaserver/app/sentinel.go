package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

import (
	"github.com/AlexStocks/goext/database/redis"
	"github.com/AlexStocks/log4go"
	"github.com/pkg/errors"
)

type (
	ClusterMeta struct {
		Version   int32                       `json:"version, omitempty"`
		Instances map[string]gxredis.Instance `json:"instances, omitempty"`
	}
	SentinelWorker struct {
		sntl *gxredis.Sentinel
		// redis instances meta data
		sync.RWMutex
		meta    ClusterMeta
		wg      sync.WaitGroup
		watcher *gxredis.SentinelWatcher
	}
)

func NewSentinelWorker() *SentinelWorker {
	worker := &SentinelWorker{
		sntl: gxredis.NewSentinel(Conf.Redis.Sentinels),
		meta: ClusterMeta{
			Instances: make(map[string]gxredis.Instance, 32),
		},
	}

	instances, err := worker.sntl.GetInstances()
	if err != nil {
		panic(fmt.Sprintf("st.GetInstances, error:%#v\n", err))
	}

	for _, inst := range instances {
		// discover new sentinel
		err = worker.sntl.Discover(inst.Name)
		if err != nil {
			panic(fmt.Sprintf("failed to discover sentiinels of instance:%s, error:%#v", inst.Name, err))
		}
	}
	// TODO: get meta version from redis
	worker.meta.Version = 1

	return worker
}

func (w *SentinelWorker) updateClusterMeta() error {
	instances, err := w.sntl.GetInstances()
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("st.GetInstances, error:%#v\n", err))
	}

	var flag bool
	for _, inst := range instances {
		// discover new sentinel
		err = worker.sntl.Discover(inst.Name)
		if err != nil {
			return errors.Wrapf(err, "failed to discover sentiinels of instance:%s, error:%#v", inst.Name, err)
		}
		slaves := inst.Slaves
		// delete unavailable slave
		inst.Slaves = inst.Slaves[:0]
		for _, slave := range slaves {
			if slave.Available() {
				inst.Slaves = append(inst.Slaves, slave)
			}
		}

		w.RLock()
		redisInst, ok := w.meta.Instances[inst.Name]
		w.RUnlock()
		if ok { // 在原来name已经存在的情况下，再查验instance值是否相等
			instJson, _ := json.Marshal(inst)
			redisInstJson, _ := json.Marshal(redisInst)
			ok = string(instJson) == string(redisInstJson)
		}
		if !ok {
			w.Lock()
			flag = true
			w.meta.Instances[inst.Name] = inst
			w.Unlock()
		}
	}
	if flag {
		w.Lock()
		w.meta.Version++
		w.Unlock()
		// update meta data to meta redis
	}

	return nil
}

func (w *SentinelWorker) updateClusterMetaByInstanceSwitch(info gxredis.MasterSwitchInfo) {
	w.Lock()
	defer w.Unlock()
	inst, ok := w.meta.Instances[info.Name]
	inst.Name = info.Name
	inst.Master = info.NewMaster
	if !ok {
		w.meta.Instances[inst.Name] = inst
		w.meta.Version++
		return
	}

	for i, slave := range inst.Slaves {
		if string(slave.IP) == string(info.NewMaster.IP) && slave.Port == info.NewMaster.Port {
			inst.Slaves = append(inst.Slaves[:i], inst.Slaves[i+1:]...)
			break
		}
	}
	w.meta.Version++
}

func (w *SentinelWorker) WatchInstanceSwitch() error {
	var (
		err error
	)
	w.watcher, err = w.sntl.MakeSentinelWatcher()
	if err != nil {
		return errors.Wrapf(err, "MakeSentinelWatcher")
	}
	c, _ := w.watcher.Watch()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for addr := range c {
			Log.Info("redis instance switch info: %#v\n", addr)
			w.updateClusterMetaByInstanceSwitch(addr)
		}
		Log.Info("instance switch watch exit")
	}()

	return nil
}

func (w *SentinelWorker) Close() {
	w.watcher.Close()
	w.wg.Wait()
	w.sntl.Close()
}
