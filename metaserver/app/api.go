package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
)

type Response struct {
	Status  int
	Message string
}

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
	//if id, ok := r.Header[textproto.CanonicalMIMEHeaderKey("id")]; ok {
	//}

	meta, err := json.Marshal(worker.meta)
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Status: -2, Message: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(&Response{Status: 0, Message: string(meta)})
}

// startHTTP start a HTTP server to serve.
func startHTTP(addr string) {
	http.HandleFunc("/stack", dumpStackHandler)
	http.HandleFunc("/cluster/meta", getMetaHandler)
	Log.Critical(http.ListenAndServe(addr, LogMiddleware(http.DefaultServeMux)))
}
