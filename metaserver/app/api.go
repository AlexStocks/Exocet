package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"runtime"
	"strconv"
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

	// If open up the authentication
	if sched.Config.API.EnabledAuth {
		Log.Info("from ip %v", ip)
		if !Authenticate(ip, sched.APIWhiteList) {
			json.NewEncoder(w).Encode(&Response{Status: -2, Message: "Access denied"})
			return
		}
	}

	if r.Body == nil {
		json.NewEncoder(w).Encode(&Response{Status: -1, Message: "Empty payload"})
		return
	}

	var plan Plan
	err = json.NewDecoder(r.Body).Decode(&plan)
	if err != nil {
		json.NewEncoder(w).Encode(&Response{Status: -1, Message: fmt.Sprintf("%v", err)})
		return
	}

	if requestID, ok := r.Header[textproto.CanonicalMIMEHeaderKey("push-id")]; ok && 0 < len(requestID) {
		plan.RequestID = requestID[0]
	}

	if pushExpiration, ok := r.Header[textproto.CanonicalMIMEHeaderKey("push-expiration")]; ok {
		if expiration, err := strconv.Atoi(pushExpiration[0]); err == nil {
			plan.RequestExpiration = int64(expiration)
		}
	}

	Log.Info("receive http request, ip:%v, plan:%#v", ip, plan)
	go sched.Handle(&plan)
	json.NewEncoder(w).Encode(&Response{Status: 0, Message: "Plan is scheduled"})
}

// startHTTP start a HTTP server to serve.
func startHTTP(addr string) {
	http.HandleFunc("/stack", dumpStackHandler)
	http.HandleFunc("/cluster/getmeta", getMetaHandler)
	Log.Critical(http.ListenAndServe(addr, LogMiddleware(http.DefaultServeMux)))
}
