package rpc

import (
	"log"
	"net"
)

type RPCServer struct {
	rpc  *RPC
	conn chan net.Conn
}

func (rs *RPCServer) startListen() {
	lsn, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("fail to listen:", err)
	}
	defer lsn.Close()

	for {
		conn, err := lsn.Accept()
		if err != nil {
			log.Println("fail to listen: ", err)
		}

		rs.conn <- conn
	}
}

func (rs *RPCServer) handleConnect(conn net.Conn) {
	defer conn.Close()
}

func (rs *RPCServer) Call(service string, request interface{}, reply interface{}) error {
	return nil
}

func (rs *RPCServer) Connect(addr string, servName string) error {
	conn, err := net.Dial("tcp", servName)
	if err != nil {
		return err
	}

	conn.Write([]byte(servName))

	return nil
}

func (rs *RPCServer) Start(listen string) {

}

func (rs *RPCServer) Run() {
	go rs.startListen()
	for {
		select {
		case conn := <-rs.conn:
			go rs.handleConnect(conn)
		}
	}
}
