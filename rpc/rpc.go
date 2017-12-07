package rpc

import (
	"encoding/gob"
	"bytes"
	"strings"
	"fmt"
	"time"
	"sync"
	"log"
	"reflect"
	_ "math/rand"
)

type reqData struct {
	endNode 	interface{}  
	servInf 	string  		// rpc接口
	argsType 	reflect.Type  	// 参数类型
	args 		[]byte 			// 传入参数
	replyCh 	chan replyBuf
}

type replyBuf struct {
	result 		bool
	reply 		[]byte
} 

type Network struct {
	mux 			sync.Mutex
	reliable		bool 		// 是否
	ends 			map[interface{}]*ClientEnd  // 存放客户端
	Servers 		map[interface{}]*Server  // 存放服务
	connections 	map[interface{}]interface{} // 连接信息
	enables 		map[interface{}]bool 
	reqCh 		    chan reqData
}

func NewNetwork() *Network {
	net := &Network{
		reliable: true,
		ends: map[interface{}]*ClientEnd{},
		Servers: map[interface{}]*Server{},
		connections: map[interface{}](interface{}){},
		enables: map[interface{}]bool{},  // 服务是否生效
		reqCh: make(chan reqData),
	}

	// 处理消息调用
	go func() {
		for req := range net.reqCh {
			go net.RequestProcess(req)
		}
	}()

	return net
}

func (nw *Network) NewClientEnd(name interface{}) *ClientEnd {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	if _, ok := nw.ends[name]; ok {
		log.Fatalf("clientEnd:%v does exist\n", name)
	}

	client := &ClientEnd{
		endNode: name,
		reqCh: nw.reqCh,
	}

	nw.ends[name] = client 
	nw.enables[name] = false 
	nw.connections[name] = nil

	return client 
}

// 添加服务
func (nw *Network) AddServer(servName interface{}, serv *Server) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.Servers[servName] = serv
}

// 删除服务
func (nw *Network) DelServer(servName interface{}) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.Servers[servName] = nil
}

func (nw *Network) Reliable(reliable bool) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.reliable = reliable
}

func (nw *Network) Enable(servName string, enable bool) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.enables[servName] = enable 
}

func (nw *Network) IsServerDead(nodeName interface{}, servName interface{}, Server *Server) bool {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	if nw.enables[nodeName] == false || nw.Servers[servName] != Server {
		return true 
	}
	return false
}

func (nw *Network) Connect(endNode interface{}, servName interface{}) {
	nw.mux.Lock()
	defer nw.mux.Unlock()

	nw.connections[endNode] = servName
}

func (nw *Network) RequestProcess(req reqData) {
	enable := nw.enables[req.endNode]
	servName := nw.connections[req.endNode]
	// reliable := nw.reliable

	if enable && servName != nil  {
		fmt.Println("begin process request")
		Server := nw.Servers[servName]
		if Server == nil {
			return 
		}

		echo := make(chan replyBuf)
		go func() {
			reply := Server.dispatch(req)
			echo <- reply
		}()

		var rep replyBuf
		ServerStatus := false 
		replyStatus := false 
		for ServerStatus == false && replyStatus == false {
			select {
			case rep = <- echo:
				replyStatus = true 
			case <-time.After(100 * time.Millisecond):
				ServerStatus = nw.IsServerDead(req.endNode, servName, Server)
			}
		}
		// fmt.Println("reply :", rep)
		req.replyCh <- rep 
	} else {
		log.Printf("Server node %s not enable\n", req.endNode)
		req.replyCh <- replyBuf{false, nil}
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

func (srv *Server) dispatch(req reqData) replyBuf {
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
		return replyBuf{false, nil}
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

func (svc *Service) dispatch(methodName string, req reqData) replyBuf {
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

		return replyBuf{true, rb.Bytes()}
	} else {
		return replyBuf{false, nil}
	}
}

type ClientEnd struct {
	endNode 	interface{}
	reqCh 		chan reqData
}

func (end *ClientEnd) Call(servInf string, args interface{}, reply interface{}) bool {
	req := reqData {
		endNode: end.endNode,
		servInf: servInf,
		argsType: reflect.TypeOf(args),
		replyCh: make(chan replyBuf),
	}

	// 将args编码转化为bytes
	var buffer bytes.Buffer
	encBuf := gob.NewEncoder(&buffer)
	encBuf.Encode(args)
	req.args = buffer.Bytes()

	end.reqCh <- req  
	rep := <-req.replyCh
	if rep.result {
		// 将返回参数解码 
		replyBuf := bytes.NewBuffer(rep.reply)
		decBuf := gob.NewDecoder(replyBuf)
		if err := decBuf.Decode(reply); err != nil {
			log.Fatalf("Client call fail: decode reply: %v\n", err)
		}

		return true 
	} else {
		return false
	}
	
}

