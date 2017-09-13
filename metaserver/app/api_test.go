package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

import (
	"github.com/AlexStocks/goext/database/redis"
	"github.com/golang/protobuf/proto"
)

const (
	HOST_IP = "192.168.11.100"
)

func Test_addInstanceHandler(t *testing.T) {
	//st := gxredis.NewSentinel(
	//	[]string{HOST_IP + ":26380", HOST_IP + ":26381", HOST_IP + ":26382"},
	//)
	//defer st.Close()
	//st.RemoveInstance("cache1")
	inst := gxredis.RawInstance{
		Name: "cache1",
		Addr: &gxredis.IPAddr{
			IP:   HOST_IP,
			Port: 4001,
		},
		Epoch:           2,
		Sdowntime:       15,
		FailoverTimeout: 450,
	}

	reqString, err := proto.Marshal(&inst)
	if err != nil {
		t.Fatalf("proto.Marshal error: %#v", err)
		t.FailNow()
	}

	reqBody := bytes.NewBuffer([]byte(reqString))
	rsp, err := http.Post("http://localhost:10080/cluster/addInstance", "application/protobuf;charset=utf-8", reqBody)
	if err != nil {
		t.Fatalf("http.Post error: %#v", err)
		t.FailNow()
	}
	result, err := ioutil.ReadAll(rsp.Body)
	defer rsp.Body.Close()
	if err != nil {
		t.Fatalf("response error: %#v", err)
	}
	var addRsp Response
	json.Unmarshal(result, &addRsp)
	if addRsp.Code != EC_OK {
		t.Fatalf("response: %#v", addRsp)
		t.FailNow()
	}
	t.Logf("response: %#v", addRsp)
}

func Test_addInstanceHandlerIllegalAddr(t *testing.T) {
	st := gxredis.NewSentinel(
		[]string{HOST_IP + ":26380", HOST_IP + ":26381", HOST_IP + ":26382"},
	)
	defer st.Close()
	st.RemoveInstance("cache1")

	inst := gxredis.RawInstance{
		Name: "cache1",
		Addr: &gxredis.IPAddr{
			IP:   HOST_IP,
			Port: 84007,
		},
		Epoch:           2,
		Sdowntime:       15,
		FailoverTimeout: 450,
	}

	reqString, err := proto.Marshal(&inst)
	if err != nil {
		t.Fatalf("proto.Marshal error: %#v", err)
		t.FailNow()
	}

	reqBody := bytes.NewBuffer([]byte(reqString))
	rsp, err := http.Post("http://localhost:10080/cluster/addInstance", "application/protobuf;charset=utf-8", reqBody)
	if err != nil {
		t.Fatalf("http.Post error: %#v", err)
		t.FailNow()
	}
	result, err := ioutil.ReadAll(rsp.Body)
	defer rsp.Body.Close()
	if err != nil {
		t.Fatalf("response error: %#v", err)
	}
	var addRsp Response
	json.Unmarshal(result, &addRsp)
	if addRsp.Code != EC_ILLEGAL_PARAM {
		t.Fatalf("response: %#v", addRsp)
		t.FailNow()
	}
	t.Logf("response: %#v", addRsp)
}

func Test_removeInstanceHandler(t *testing.T) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://localhost:10080/cluster/removeInstance", nil)
	if err != nil {
		t.Fatalf("http.NewRequest error: %#v", err)
		t.FailNow()
	}
	req.Header.Set("Content-Type", "application/protobuf;charset=utf-8")
	req.Header.Set("Instance-Name", "cache1")
	rsp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http.Post error: %#v", err)
		t.FailNow()
	}
	result, err := ioutil.ReadAll(rsp.Body)
	defer rsp.Body.Close()
	if err != nil {
		t.Fatalf("response error: %#v", err)
	}
	var delRsp Response
	json.Unmarshal(result, &delRsp)
	if delRsp.Code != EC_OK {
		t.Fatalf("response: %#v", delRsp)
		t.FailNow()
	}
	t.Logf("removeInstace response: %#v", delRsp)

	//// add instance
	//st := gxredis.NewSentinel(
	//	[]string{HOST_IP + ":26380", HOST_IP + ":26381", HOST_IP + ":26382"},
	//)
	//defer st.Close()
	//
	//inst := gxredis.RawInstance{
	//	Name: "cache1",
	//	Addr: &gxredis.IPAddr{
	//		IP:   HOST_IP,
	//		Port: 4001,
	//	},
	//	Epoch:           2,
	//	Sdowntime:       15,
	//	FailoverTimeout: 450,
	//}
	//st.AddInstance(inst)
}
