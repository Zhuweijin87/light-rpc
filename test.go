package main

import (
	"light-rpc/rpc"
	"runtime"
	"sync"
	"fmt"
	"strconv"
)

type TestServer struct {
	mux 	sync.Mutex
	Data 	[]string 
	Count 	[]int 
}

func (ts *TestServer) HandleFunc_01(args int, reply *string) {
	ts.mux.Lock()
	defer ts.mux.Unlock()

	ts.Count = append(ts.Count, args)
	*reply = "handle-" + strconv.Itoa(args)
}

func (ts *TestServer) HandleFunc_02(args string, reply *string) {
	ts.mux.Lock()
	defer ts.mux.Unlock()

	*reply = "handle-string" + args
}

func main() {
	runtime.GOMAXPROCS(4)
	network := rpc.NewNetwork()
	node := network.NewClientEnd("node_01")

	ts := &TestServer{}
	service := rpc.NewService(ts)

	server := rpc.NewServer()
	server.AddService(service)
	network.AddServer("server_01", server)

	network.Connect("node_01", "server_01")

	network.Enable("node_01", true)

	reply := ""
	node.Call("TestServer.HandleFunc_01", 2100, &reply)
	fmt.Println("reply: ", reply)
	
	reply = ""
	node.Call("TestServer.HandleFunc_02", "asafaer", &reply)
	fmt.Println("reply: ", reply)
}