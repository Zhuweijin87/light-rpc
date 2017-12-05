package main

import (
	"sync"
	"reflect"
)

type reqData struct {
	endNode 	interface{}  
	servInf 	string  		// rpc接口
	argsType 	reflect.Type  	// 参数类型
	args 		[]byte 			// 传入参数
	replyCh 	chan replyData
}

type replyData struct {
	result 		bool
	reply 		[]byte
} 

type ClientEnd struct {
	endNode 	interface{}
	reqCh 		chan reqData
}

// servInf:
// args:
// reply:
func (e *ClientEnd) Call(servInf string, args interface{}, reply interface{}) bool {
	
	return true 
}

type Network struct {
	mux 			sync.Mutex
	reliable		bool 		// 是否
	ends 			map[interface{}]*ClientEnd  // 存放客户端
	servers 		map[interface{}]*Server  // 存放服务
	connections 	map[interface{}]interface{} // 连接信息
	enables 		map[interface{}]bool 
}

func NewNetwork() *Network {
	net := &Network{
		reliable: true,
		ends: map[interface{}]*ClientEnd{},
		servers: map[interface{}]*Server{},
		connections: map[interface{}](interface{}){},
		enables: map[interface{}]bool{},
	}

	return net
}

func (nw *Network) Reliable(reliable bool) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.reliable = reliable
}

func (nw *Network) IsServerDead(nodeName interface{}, servName interface{}, server *Server) bool {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	if nw.enables[nodeName] == false || nw.servers[servName] != server {
		return true 
	}
	return false
}

func (nw *Network) RequestProcess(req reqData) {
	
}

type Server struct {

}