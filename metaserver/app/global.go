package main

import (
	"sync/atomic"
	"time"
)

import (
	"github.com/AlexStocks/goext/log"
)

type (
	empty interface{}
)

var (
	// local ip
	LocalIP   string
	LocalHost string
	// progress id
	ProcessID string
	// Conf is main config
	Conf ConfYaml
	// Log records server request log
	Log gxlog.Logger
	// now in unix time
	Now int64
)

func UpdateNow() {
	atomic.StoreInt64(&Now, int64(time.Now().Unix()))
}
