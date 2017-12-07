package rpc2

import (
	"strings"
	"fmt"
	"encoding/gob"
	"bytes"
	"reflect"
	"sync"
	"log"
)

type serverRegister  	map[string]*Server 
type endpointRegister 	map[string]*Endpoint 
type connectRegister	map[string]interface{} 
type enableRegister 	map[string]bool 

type RPC struct {
	mux 			sync.Mutex
	servers 		serverRegister
	endpoints 		endpointRegister 
	connections 	connectRegister	
	enables			enableRegister
	requestCh 		chan requestData	
}

type requestData struct {
	servInf 		string 
	argsType 		reflect.Type
	args 			[]byte 
	replyCh 		chan replyData
}

type replyData struct {
	result 			bool 
	err 			error 
	reply 			[]byte 
}

func NewRPC() *RPC {
	rpc := RPC {
		servers: make(serverRegister),
		endpoints: make(endpointRegister),
		connections: make(connectRegister),
		enables: make(enableRegister),
		requestCh: make(chan requestData),
	}

	// 处理请求数据
	go func() {
		for request := range rpc.requestCh {
			go rpc.ProcessRequest(request)
		}
	}()

	return &rpc
}

func (rpc *RPC) ProcessRequest(req requestData) {

}

// 添加服务
func (rpc *RPC) Add(servName string, serv *Server) {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	rpc.servers[servName] = serv 
}

// 删除服务
func (rpc *RPC) Del(servName string) {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	rpc.servers[servName] = nil 
}

type Endpoint struct {
	name 		string 
	requestCh	chan requestData
}

func (rpc *RPC) NewEndpoint(name string) *Endpoint {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	if _, ok := rpc.endpoints[name]; ok {
		log.Printf("endpoint-%s does exist\n", name)
		return nil 
	}

	edp := &Endpoint{
		name: name,
		requestCh: make(chan requestData),
	}

	rpc.endpoints[name] = edp
	rpc.enables[name] = false // 默认未开启
	rpc.connections[name] = nil 

	return edp
}

// 执行具体方法
func (edp *Endpoint) Call(servInf string, args interface{}, reply interface{}) {

}

type serviceRegister map[string]*Service 
type Server struct {
	mux 		sync.Mutex
	services 	serviceRegister
}

func NewServer() *Server {
	return &Server{
		services: make(serviceRegister),
	}
}

func (srv *Server) Add(svc *Service) {
	srv.mux.Lock()
	defer srv.mux.Unlock()

	srv.services[svc.name] = svc 
}

func (srv *Server) dispatch(req requestData) replyData {
	srv.mux.Lock()
	dot := strings.LastIndex(req.servInf, ".")
	serviceName := req.servInf[:dot]
	methodName := req.servInf[dot+1:]
	service, ok := srv.services[serviceName]
	defer srv.mux.Unlock()

	if ok {
		return service.dispatch(methodName, req)
	} else {
		return replyData{false, nil, nil}
	}
}

type methodRegister map[string]reflect.Method
type Service struct {
	name 		string 
	refType 	reflect.Type 
	refValue	reflect.Value
	methods 	methodRegister
}

func NewService(servInf interface{}) *Service {
	var service Service 
	service.refType = reflect.TypeOf(servInf)
	service.refValue = reflect.ValueOf(servInf)
	service.name = reflect.Indirect(service.refValue).Type().Name()
	service.methods = make(methodRegister)
	
	numMethod := service.refType.NumMethod()
	for i:=0; i<numMethod; i++ {
		method := service.refType.Method(i)
		methType := method.Type
		methName := method.Name

		// 传入参数为2两个, 且最后一个参数为指针， 传出参数为0
		if methType.NumIn() != 3 || methType.In(2).Kind() != reflect.Ptr || methType.NumOut() != 0 {
			// error 
			log.Printf("wrong method %v\n", methName)
		} else {
			service.methods[methName] = method
		}
	}
	return &service 
}

func (svc *Service) dispatch(methName string, req requestData) replyData {
	if method, ok := svc.methods[methName]; ok {
		args := reflect.New(req.argsType)
		argsBuf := bytes.NewBuffer(req.args)
		argsDec := gob.NewDecoder(argsBuf)

		// 将参数解码出来
		argsDec.Decode(args.Interface())

		// 取返回参数
		replyArgs := method.Type.In(2).Elem()
		replyVal := reflect.New(replyArgs)

		// 反射回调执行
		method.Func.Call([]reflect.Value{svc.refValue, args.Elem(), replyVal})

		// 解析结果 
		resultBuf := new(bytes.Buffer)
		gob.NewDecoder(resultBuf).DecodeValue(replyVal)

		return replyData{true, nil, resultBuf.Bytes()}
	} else {
		return replyData{false, fmt.Errorf("method does not exist"), nil}
	}
}


