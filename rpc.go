package main

import (
	"encoding/gob"
	"bytes"
	"strings"
	"time"
	"sync"
	"reflect"
	"math/rand"
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
	req := reqData {
		endNode: e.endNode,
		servInf: servInf,
		argsType: reflect.TypeOf(args),
		replyCh: make(chan replyData),
	}

	buffer := new(bytes.Buffer)
	gobBuffer := gob.NewEncoder(buffer)
	gobBuffer.Encode(args)
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

// 添加服务
func (nw *Network) AddServer(servName interface{}, serv *Server) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.servers[servName] = serv
}

// 删除服务
func (nw *Network) DelServer(servName interface{}) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.servers[servName] = nil
}

func (nw *Network) Connect(endNode interface{}, servName interface{}) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.connections[endNode] = servName
}

func (nw *Network) RequestProcess(req reqData) {
	enable := nw.enables[req.endNode]
	servName := nw.connections[req.endNode]
	reliable := nw.reliable

	if enable && servName != nil  {
		server := nw.servers[servName]
		if server == nil {
			return 
		}

		if reliable == false {
			ms := rand.Int() % 27
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}

		if reliable == false && (rand.Int()%1000) < 100 {
			// 丢弃请求数据 
			req.replyCh <- replyData{false, nil}
			return 
		}

		echo := make(chan replyData)
		go func() {
			reply := server.dispatch(req)
			echo <- reply
		}()

		//var reply replyData

	}
}

type Server struct {
	mux 		sync.Mutex
	services 	map[string]*Service 
	count 		int 	// 统计接收的RPC
}

func NewServer() *Server {
	return &Server {
		services: map[string]*Service{},
		count: 0,
	}
}

func (srv *Server) AddService(svc *Service) {
	srv.mux.Lock()
	defer srv.mux.Unlock()

	srv.services[svc.name] = svc
}

func (srv *Server) dispatch(req reqData) replyData {
	srv.mux.Lock()
	srv.count += 1
	dot := strings.LastIndex(req.servInf, ".")
	serviceName := req.servInf[:dot]
	methodName := req.servInf[dot+1:]

	service, ok := srv.services[serviceName]
	srv.mux.Unlock()

	if ok {
		// 执行service方法
		return service.dispatch(methodName, req)
	} else {
		return replyData{false, nil}
	}
}

type Service struct {
	name 		string
	refValue 	reflect.Value
	refTyp		reflect.Type
	methods		map[string]reflect.Method		
}

func NewService(inf interface{}) *Service {
	var service Service
	service.refTyp = reflect.TypeOf(inf)
	service.refValue = reflect.ValueOf(inf)
	service.name = reflect.Indirect(service.refValue).Type().Name()
	service.methods = map[string]reflect.Method{}

	for i := 0; i < service.refTyp.NumMethod(); i++ {
		method := service.refTyp.Method(i)
		mtype := method.Type
		mname := method.Name 

		if method.PkgPath != "" || mtype.NumIn() != 3 || mtype.In(2).Kind() != reflect.Ptr || mtype.NumOut() != 0 {
			// TODO 
		} else {
			service.methods[mname] = method
		}
	}

	return &service
}

func (svc *Service) dispatch(methodName string, req reqData) replyData {
	if method, ok := svc.methods[methodName]; ok {
		args := reflect.New(req.argsType)
		argsBuf := bytes.NewBuffer(req.args)
		argsData := gob.NewDecoder(argsBuf)

		argsData.Decode(args.Interface())

		replyType := method.Type.In(2) // 取返回的参数
		replyType = replyType.Elem()
		replyVal := reflect.New(replyType)

		// 通过函数反射的方式调用执行方法
		function := method.Func
		function.Call([]reflect.Value{svc.refValue, args.Elem(), replyVal})

		rb := new(bytes.Buffer)
		re := gob.NewEncoder(rb)
		re.EncodeValue(replyVal)

		return replyData{true, rb.Bytes()}
	} else {
		return replyData{false, nil}
	}
}
