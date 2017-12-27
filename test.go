package main

import (
	"fmt"
	rpc "light-rpc/rpc"
	"runtime"
	"strconv"
	"sync"
)

type TestServer struct {
	mux   sync.Mutex
	Data  []string
	Count []int
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

func rpc01() {
	runtime.GOMAXPROCS(4)
	rpcServ := rpc.NewRPC()

	server := rpc.NewServer()
	rpcServ.Add("server-01", server)

	ts := &TestServer{}
	service := rpc.NewService(ts)
	server.Add(service)

	node := rpcServ.NewEndpoint("node-01")
	rpcServ.Connect("node-01", "server-01")

	reply := ""
	node.Call("TestServer.HandleFunc_01", 2101, &reply)
	fmt.Println("reply:", reply)

	node2 := rpcServ.NewEndpoint("node-02")
	rpcServ.Connect("node-02", "server-01")

	reply = ""
	node2.Call("TestServer.HandleFunc_02", "Bingo", &reply)
	fmt.Println("reply:", reply)
}

func main() {
	rpc01()
}
