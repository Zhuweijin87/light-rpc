package rpc

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"
)

type serverRegister map[string]*Server
type endpointRegister map[string]*Endpoint
type connectRegister map[string]string // 连接信息
type enableRegister map[string]bool    // 是否开启

type RPC struct {
	mux         sync.Mutex
	servers     serverRegister
	endpoints   endpointRegister
	connections connectRegister
	enables     enableRegister
	requestCh   chan requestData // 所有数据请求
}

type requestData struct {
	epnode   string // 节点信息
	servInf  string // 请求服务接口
	argsType reflect.Type
	args     []byte // 参数
	replyCh  chan replyData
}

type replyData struct {
	result bool
	err    error
	reply  []byte
}

func NewRPC() *RPC {
	rpc := RPC{
		servers:     make(serverRegister),
		endpoints:   make(endpointRegister),
		connections: make(connectRegister),
		enables:     make(enableRegister),
		requestCh:   make(chan requestData),
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
	log.Printf("servInf: %s\n", req.servInf)
	servName, ok := rpc.connections[req.epnode]
	if !ok {
		log.Printf("endnode %s not connect\n", req.epnode)
		req.replyCh <- replyData{false, nil, nil}
		return
	}

	server, ok := rpc.servers[servName]
	if !ok {
		log.Printf("server %s not exist\n", servName)
		req.replyCh <- replyData{false, nil, nil}
		return
	}

	echo := make(chan replyData)
	go func() {
		// 处理服务
		reply := server.dispatch(req)
		echo <- reply
	}()

	var replyed replyData
	statusCheck := false
	statusEcho := false
	for statusEcho == false && statusCheck == false {
		select {
		case replyed = <-echo:
			statusEcho = true
		case <-time.After(100 * time.Millisecond):
			statusCheck = rpc.isServerDead()
		}
	}

	req.replyCh <- replyed
}

// 判断服务状态
func (rpc *RPC) isServerDead() bool {
	return false
}

// 添加服务
func (rpc *RPC) Add(servName string, serv *Server) {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	rpc.servers[servName] = serv
}

// 删除服务
func (rpc *RPC) Delete(servName string) {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	rpc.servers[servName] = nil
}

// 连接
func (rpc *RPC) Connect(epName, servName string) {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	rpc.connections[epName] = servName
}

type Endpoint struct {
	name      string
	requestCh chan requestData
}

func (rpc *RPC) NewEndpoint(name string) *Endpoint {
	rpc.mux.Lock()
	defer rpc.mux.Unlock()

	// 判断服务是否存在
	if _, ok := rpc.endpoints[name]; ok {
		log.Printf("endpoint-%s does exist\n", name)
		return nil
	}

	ep := &Endpoint{
		name:      name,
		requestCh: rpc.requestCh,
	}

	rpc.endpoints[name] = ep
	rpc.enables[name] = false // 默认未开启

	return ep
}

// 执行具体方法
func (ep *Endpoint) Call(servInf string, args interface{}, reply interface{}) bool {
	request := requestData{
		epnode:   ep.name,
		servInf:  servInf,
		argsType: reflect.TypeOf(args),
		replyCh:  make(chan replyData),
	}

	// 将args编码转化为bytes
	var buffer bytes.Buffer
	encBuf := gob.NewEncoder(&buffer)
	encBuf.Encode(args)
	request.args = buffer.Bytes()

	// 发送
	ep.requestCh <- request
	// 接收
	replyd := <-request.replyCh
	if replyd.result {
		replyBuf := bytes.NewBuffer(replyd.reply)
		decBuf := gob.NewDecoder(replyBuf)
		fmt.Println("")
		if err := decBuf.Decode(reply); err != nil {
			log.Fatalf("Client call fail: decode reply: %v\n", err)
		}

		return true
	} else {
		return false
	}
}

type serviceRegister map[string]*Service
type Server struct {
	mux      sync.Mutex
	services serviceRegister
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
	name     string
	refType  reflect.Type
	refValue reflect.Value
	methods  methodRegister
}

func NewService(servInf interface{}) *Service {
	var service Service
	service.refType = reflect.TypeOf(servInf)
	service.refValue = reflect.ValueOf(servInf)
	service.name = reflect.Indirect(service.refValue).Type().Name()
	service.methods = make(methodRegister)

	numMethod := service.refType.NumMethod()
	for i := 0; i < numMethod; i++ {
		method := service.refType.Method(i)
		methType := method.Type
		methName := method.Name

		// 条件: 传入参数为2两个, 且最后一个参数为指针， 传出参数为0
		// methType.In(0): 函数本身作为参数
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
	log.Println("method dispatch:", methName)
	if method, ok := svc.methods[methName]; ok {
		args := reflect.New(req.argsType)
		argsBuf := bytes.NewBuffer(req.args)
		argsDec := gob.NewDecoder(argsBuf)

		// 将参数解码出来
		argsDec.Decode(args.Interface())

		// 取返回参数
		replyArgs := method.Type.In(2).Elem()
		replyVal := reflect.New(replyArgs)

		// 反射回调函数执行
		method.Func.Call([]reflect.Value{svc.refValue, args.Elem(), replyVal})

		// 执行后返回结果encode
		resultBuf := new(bytes.Buffer)
		gob.NewEncoder(resultBuf).EncodeValue(replyVal)

		return replyData{true, nil, resultBuf.Bytes()}
	} else {
		return replyData{false, fmt.Errorf("method does not exist"), nil}
	}
}
