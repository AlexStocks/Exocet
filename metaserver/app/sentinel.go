package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

import (
	"github.com/AlexStocks/goext/database/redis"
	"github.com/AlexStocks/goext/math/rand"
	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
)

type (
	SentinelWorker struct {
		sntl *gxredis.Sentinel
		// redis instances meta data
		sync.RWMutex
		meta          ClusterMeta
		wg            sync.WaitGroup
		switchWatcher *gxredis.SentinelWatcher
		sdownWatcher  *gxredis.SentinelWatcher
	}
)

func NewSentinelWorker() *SentinelWorker {
	var (
		err       error
		instances []gxredis.Instance
		metaDB    gxredis.Instance
		sw        *SentinelWorker
	)

	sw = &SentinelWorker{
		sntl: gxredis.NewSentinel(Conf.Redis.Sentinels),
		meta: ClusterMeta{
			Instances: make(map[string]*gxredis.Instance, 32),
		},
	}

	instances, err = sw.sntl.GetInstances()
	if err != nil {
		panic(fmt.Sprintf("st.GetInstances, error:%#v\n", err))
	}

	for _, inst := range instances {
		if inst.Name == Conf.Redis.MetaDBName {
			metaDB = inst
		}
		// discover new sentinel
		err = sw.sntl.Discover(inst.Name, []string{"127.0.0.1"})
		if err != nil {
			panic(fmt.Sprintf("failed to discover sentiinels of instance:%s, error:%#v", inst.Name, err))
		}
	}

	// sw.meta.Version = 0
	if metaDB.Name == "" {
		panic("can not find meta db.")
	}
	if err = sw.loadClusterMetaData(); err != nil {
		panic(fmt.Sprintf("loadClusterMetaData() = error:%#v", err))
	}
	Log.Debug("after loadClusterMetaData(), worker.meta:%s", sw.meta.Instances)
	sw.updateClusterMeta()
	Log.Debug("after updateClusterMetaData(), worker.meta:%s", sw.meta.Instances)

	return sw
}

func (w *SentinelWorker) loadClusterMetaData() error {
	var (
		err       error
		res       interface{}
		instances []gxredis.Instance
		metaDB    gxredis.Instance
		metaConn  redis.Conn
		key       string
		value     []byte
		version   int
	)

	instances, err = w.sntl.GetInstances()
	if err != nil {
		return fmt.Errorf("st.GetInstances, error:%#v\n", err)
	}

	for _, inst := range instances {
		if inst.Name == Conf.Redis.MetaDBName {
			metaDB = inst
			break
		}
	}

	if metaConn, err = w.sntl.GetConnByRole(metaDB.Master.TcpAddr().String(), gxredis.RR_Master); err != nil {
		return errors.Wrapf(err, "gxsentinel.GetConnByRole(%s, RR_Master)", metaDB.Master.TcpAddr().String())
	}
	defer metaConn.Close()

	if res, err = metaConn.Do("hgetall", Conf.Redis.MetaHashtable); err != nil {
		return errors.Wrapf(err, "hgetall(%s)", Conf.Redis.MetaHashtable)
	}
	if res != nil {
		arr := res.([]interface{})
		for _, elem := range arr {
			if len(key) == 0 {
				key = string(elem.([]byte))
				continue
			}

			value = elem.([]byte)
			if key == Conf.Redis.MetaVersion {
				if version, err = strconv.Atoi(string(value)); err != nil {
					return errors.Wrapf(err, "strconv.Atoi(%s)", string(value))
				}
				w.meta.Version = int32(version)
			} else if key == Conf.Redis.MetaInstNameList {
			} else {
				var inst gxredis.Instance
				if err = json.Unmarshal(value, &inst); err != nil {
					return errors.Wrapf(err, "json.Unmarshal(value:%s)", string(value))
				}
				Log.Debug("name:%s, inst:%s", key, inst)
				w.meta.Instances[key] = &inst
			}
			key = ""
		}
	}

	return nil
}

func (w *SentinelWorker) storeClusterMetaData() error {
	var (
		err              error
		ok               bool
		queued           interface{}
		jsonStr          []byte
		metaDB           *gxredis.Instance
		metaConn         redis.Conn
		instanceNameList InstanceNameList
	)

	w.RLock()
	defer w.RUnlock()

	if len(w.meta.Instances) == 0 {
		return fmt.Errorf("redis cluster instance pool is empty")
	}

	if metaDB, ok = w.meta.Instances[Conf.Redis.MetaDBName]; !ok {
		return fmt.Errorf("can not find meta db")
	}

	if metaConn, err = w.sntl.GetConnByRole(metaDB.Master.TcpAddr().String(), gxredis.RR_Master); err != nil {
		return errors.Wrapf(err, "gxsentinel.GetConnByRole(%s, RR_Master)", metaDB.Master.TcpAddr().String())
	}

	htName := Conf.Redis.MetaHashtable + "-" + time.Now().Format("20060102-150405") + "-" + gxrand.RandString(8)
	if _, err = metaConn.Do("hset", htName, Conf.Redis.MetaVersion, w.meta.Version); err != nil {
		return errors.Wrapf(err, "hset(%s, %s, %s)", htName, Conf.Redis.MetaVersion, w.meta.Version)
	}
	for k, v := range w.meta.Instances {
		if jsonStr, err = json.Marshal(v); err != nil {
			Log.Error("json.Marshal(%#v) = %#v", v, err)
			continue
		}
		if _, err = metaConn.Do("hset", htName, k, string(jsonStr)); err != nil {
			Log.Error(err, "hset(%s, %s, %s) = error:%#v", htName, k, string(jsonStr), err)
			continue
		}
		instanceNameList.List = append(instanceNameList.List, k)
	}
	if jsonStr, err = json.Marshal(instanceNameList); err != nil {
		return errors.Wrapf(err, "json.Marshal(%#v)", instanceNameList)
	}
	if _, err = metaConn.Do("hset", htName, Conf.Redis.MetaInstNameList, string(jsonStr)); err != nil {
		return errors.Wrapf(err, "hset(%s, %s, %s)", htName, Conf.Redis.MetaInstNameList, string(jsonStr))
	}

	// redis.tx
	defer func() {
		if err != nil {
			metaConn.Do("discard")
		}
		metaConn.Close()
	}()

	if _, err = metaConn.Do("watch", Conf.Redis.MetaHashtable); err != nil {
		return errors.Wrapf(err, "watch %s", Conf.Redis.MetaHashtable)
	}

	metaConn.Send("multi")
	if _, err = metaConn.Do("rename", htName, Conf.Redis.MetaHashtable); err != nil {
		return errors.Wrapf(err, "rename(%s, %s)", htName, Conf.Redis.MetaHashtable)
	}
	queued, err = metaConn.Do("exec")
	if err != nil {
		return errors.Wrapf(err, "exec")
	}
	if queued != nil {
		return fmt.Errorf("transaction exec result:%#v", queued)
	}

	return nil
}

func (w *SentinelWorker) updateClusterMeta() error {
	instances, err := w.sntl.GetInstances()
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("st.GetInstances, error:%#v\n", err))
	}
	Log.Debug("current meta:%s", w.meta)

	var flag bool
	for _, i := range instances {
		inst := i
		// discover new sentinel
		err = w.sntl.Discover(inst.Name, []string{"127.0.0.1"})
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
		// Log.Debug("instance %s, redisInst:%s, ok:%v", inst, redisInst, ok)
		if ok { // 在原来name已经存在的情况下，再查验instance值是否相等
			ok = inst.Equal(redisInst)
			// Log.Debug("instance{name:%s, old:%s, current:%s}, ok:%v", inst.Name, redisInst, inst, ok)
		}
		if !ok {
			w.Lock()
			Log.Debug("meta:%s, new inst:%s", w.meta, inst)
			flag = true
			w.meta.Instances[inst.Name] = &inst
			w.Unlock()
		}
	}
	if flag {
		w.Lock()
		w.meta.Version++
		w.Unlock()
		Log.Debug("current meta:%v, start to store current meta data", w.meta)
		// update meta data to meta redis
		if err = w.storeClusterMetaData(); err != nil {
			return errors.Wrapf(err, "SentinelWorker.storeClusterMetaData()")
		}
	}

	return nil
}

func (w *SentinelWorker) updateClusterMetaByInstanceSwitch(info gxredis.MasterSwitchInfo) bool {
	w.Lock()
	defer w.Unlock()
	Log.Info("got switch info:%s", info)
	inst := w.meta.Instances[info.Name]
	inst.Name = info.Name
	inst.Master = &(info.NewMaster)
	inst.Slaves = []*gxredis.Slave{}
	slaves, err := w.sntl.Slaves(inst.Name)
	if err != nil {
		Log.Error("failed to get slaves of %s", inst)
		return false
	} else {
		var slaveArray []*gxredis.Slave
		for _, slave := range slaves {
			if slave.Available() {
				slaveArray = append(slaveArray, slave)
			}
		}
		if 0 < len(slaveArray) {
			inst.Slaves = slaveArray
		}
	}

	w.meta.Instances[inst.Name] = inst
	w.meta.Version++
	Log.Debug("get switch info:%#v, new inst:%#v, version:%d", info, inst, w.meta.Version)

	return true
}

func (w *SentinelWorker) updateClusterMetaByInstanceDown(info gxredis.SdownInfo) bool {
	Log.Info("get +sdown info %s", info)
	w.Lock()
	defer w.Unlock()

	inst, ok := w.meta.Instances[info.Name]
	if !ok {
		Log.Error("cat not find instance of %s", info.Name)
		return false
	}
	if info.Role == gxredis.RR_Master {
		if inst.Master.Equal(info.Addr) {
			inst.Master = nil
			goto END
		}
	} else {
		for idx, slave := range inst.Slaves {
			if info.Addr.Equal(slave.Addr) {
				inst.Slaves = append(inst.Slaves[:idx], inst.Slaves[idx+1:]...)
				goto END
			}
		}
	}

	Log.Error("can not find info:%s in local meta", info)
	return false

END:
	if inst.Master != nil || len(inst.Slaves) != 0 {
		w.meta.Instances[inst.Name] = inst
	} else {
		delete(w.meta.Instances, inst.Name)
	}
	w.meta.Version++
	Log.Debug("sdown info:%s, new inst:%s, version:%d", info, inst, w.meta.Version)

	return true
}

func (w *SentinelWorker) WatchInstanceSwitch() error {
	var (
		err error
	)
	w.switchWatcher, err = w.sntl.MakeMasterSwitchSentinelWatcher()
	if err != nil {
		return errors.Wrapf(err, "WatchInstanceSwitch")
	}
	c, _ := w.switchWatcher.Watch()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for e := range c {
			elem := e
			info, ok := elem.(gxredis.MasterSwitchInfo)
			if !ok {
				Log.Error("%#v is not of type gxredis.MasterSwitchInfo", elem)
				continue
			}
			Log.Info("redis instance switch info: %#v\n", info)
			if w.updateClusterMetaByInstanceSwitch(info) {
				w.storeClusterMetaData()
			}
		}
		Log.Info("instance switch watch exit")
	}()

	return nil
}

func (w *SentinelWorker) WatchSdown() error {
	var (
		err error
	)
	w.sdownWatcher, err = w.sntl.MakeSdownSentinelWatcher()
	if err != nil {
		return errors.Wrapf(err, "MakeSdownSentinelWatcher")
	}
	c, _ := w.sdownWatcher.Watch()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for e := range c {
			elem := e
			info, ok := elem.(gxredis.SdownInfo)
			if !ok {
				Log.Error("%#v is not of type gxredis.SdownInfo", elem)
				continue
			}
			Log.Info("redis sentinel +sdown info: %#s\n", info)
			if w.updateClusterMetaByInstanceDown(info) {
				w.storeClusterMetaData()
			}
		}
		Log.Info("instance switch watch exit")
	}()

	return nil
}

func (w *SentinelWorker) addInstance(inst gxredis.RawInstance) error {
	return w.sntl.AddInstance(inst)
}

func (w *SentinelWorker) removeInstance(name string) error {
	return w.sntl.RemoveInstance(name)
}

func (w *SentinelWorker) Close() {
	w.switchWatcher.Close()
	w.sdownWatcher.Close()
	w.wg.Wait()
	w.sntl.Close()
}
