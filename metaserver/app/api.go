package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"runtime"
)

import (
	"github.com/AlexStocks/goext/database/redis"
	"github.com/golang/protobuf/proto"
)

// LogMiddleware access
func LogMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Log.Info("%s\t%s\t%s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

// dumpStackHandler dump the stack of goroutines into response
func dumpStackHandler(w http.ResponseWriter, r *http.Request) {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Fprintf(w, "%s", buf)
}

// getMetaHandler return the metadata of redis cluster
func getMetaHandler(w http.ResponseWriter, r *http.Request) {
	Log.Debug("get request from %#v", r.RemoteAddr)

	meta, err := json.Marshal(worker.meta)
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Code: EC_SYS_ERROR, Message: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(&Response{Code: EC_OK, Message: string(meta)})
}

// addInstanceHandler add one redis instance to cluster
func addInstanceHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	Log.Debug("get request from %#v, form:%#v", r.RemoteAddr, r.Form)
	if r.Method != "POST" {
		Log.Error("illegal add instance request method:%s", r.Method)
		json.NewEncoder(w).Encode(&Response{Code: EC_ILLEGAL_HTTP_METHOD, Message: r.Method})
		return
	}
	reqData, reqError := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if len(reqData) <= 0 || reqError != nil {
		json.NewEncoder(w).Encode(&Response{
			Code:    EC_ILLEGAL_PARAM,
			Message: fmt.Errorf("req.Body.Read() = {len:%d, err:%v}", len(reqData), reqError).Error(),
		})
		return
	}

	var inst gxredis.RawInstance
	err := proto.Unmarshal(reqData, &inst)
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Code: EC_ILLEGAL_PARAM, Message: err.Error()})
		return
	}
	err = inst.Validate()
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Code: EC_ILLEGAL_PARAM, Message: err.Error()})
		return
	}
	err = inst.Addr.Validate()
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Code: EC_ILLEGAL_PARAM, Message: err.Error()})
		return
	}
	err = worker.addInstance(inst)
	Log.Info("got add instance %#v request, error:%#v", inst, err)
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Code: EC_SYS_ERROR, Message: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(&Response{Code: EC_OK, Message: ErrorCode(EC_OK).String()})
}

// removeInstanceHandler removes one redis instance from cluster
func removeInstanceHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	Log.Debug("get request from %#v, form:%#v", r.RemoteAddr, r.Form)
	if r.Method != "POST" {
		Log.Error("illegal add instance request method:%s", r.Method)
		json.NewEncoder(w).Encode(&Response{Code: EC_ILLEGAL_HTTP_METHOD, Message: r.Method})
		return
	}

	instanceName := r.Header.Get(textproto.CanonicalMIMEHeaderKey("Instance-Name"))
	err := worker.removeInstance(instanceName)
	Log.Info("got remove instance %s request, error:%#v", instanceName, err)
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Code: EC_SYS_ERROR, Message: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(&Response{Code: EC_OK, Message: ErrorCode(EC_OK).String()})
}

// startHTTP start a HTTP server to serve.
func startHTTP(addr string) {
	http.HandleFunc("/stack", dumpStackHandler)
	http.HandleFunc("/cluster/meta", getMetaHandler)
	http.HandleFunc("/cluster/addInstance", addInstanceHandler)
	http.HandleFunc("/cluster/removeInstance", removeInstanceHandler)
	Log.Critical(http.ListenAndServe(addr, LogMiddleware(http.DefaultServeMux)))
}
