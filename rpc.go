package main

import (
	"log"
	"net/rpc"
)

type ClientMsg struct {
	Operation string
	FileName  string
	Content   string
}

func sendTCP(remoteIPPort string) {
	client, err := rpc.DialHTTP("tcp", remoteIPPort)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	// Synchronous call
	args := ClientMsg{"A", "B", "C"}
	var reply int
	err = client.Call("MinerHandle.Communication", args, &reply)
	if err != nil {
		log.Fatal("tcp error:", err)
	}
	if reply == 0 {
		println("Send successfully")
	}
}

func main() {
	sendTCP("127.0.0.1:8080")
}
