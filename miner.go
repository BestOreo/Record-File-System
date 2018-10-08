package main

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

var ip string
var port int

func printColorFont(color string, value string) {
	if color == "red" {
		fmt.Printf("%c[1;0;31m%s%c[0m\n", 0x1B, value, 0x1B)
	} else if color == "blue" {
		fmt.Printf("%c[1;0;34m%s%c[0m\n", 0x1B, value, 0x1B)
	} else if color == "purple" {
		fmt.Printf("%c[1;0;35m%s%c[0m\n", 0x1B, value, 0x1B)
	} else if color == "green" {
		fmt.Printf("%c[1;0;32m%s%c[0m\n", 0x1B, value, 0x1B)
	} else {
		println(value)
	}
}

func Server(listen *net.TCPListener) {
	for {
		conn, err := listen.AcceptTCP()
		if err != nil {
			fmt.Println(getTime())
			println("Exception on client:", err.Error())
			continue
		}
		println("---------------------")
		println(getTime())
		printColorFont("green", "connect client from "+conn.RemoteAddr().String())
		defer conn.Close()
		go func() {
			data := make([]byte, 128)
			for {
				i, err := conn.Read(data)
				if err != nil {
					fmt.Println(getTime())
					printColorFont("red", "disconnect "+conn.RemoteAddr().String())
					break
				}
				fmt.Println(getTime())
				printColorFont("blue", "Operation: "+string(data[0:i]))
				conn.Write([]byte(ip + ":" + strconv.Itoa(port) + ": get instruction \"" + string(data[0:i]) + "\""))
			}
		}()
	}
}

func getTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	ip = "127.0.0.1"
	port = 9000

	listen, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(ip), port, ""})
	if err != nil {
		fmt.Println("Fail to monitor ", err.Error())
		return
	}
	fmt.Println("Start to listen on", ip+":"+strconv.Itoa(port), "...\n")
	Server(listen)
}
