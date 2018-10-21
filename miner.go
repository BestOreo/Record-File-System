package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"
)

type configSetting struct {
	MinedCoinsPerOpBlock   int
	MinedCoinsPerNoOpBlock int
	NumCoinsPerFileCreate  int
	GenOpBlockTimeout      int
	GenesisBlockHash       string
	PowPerOpBlock          int
	PowPerNoOpBlock        int
	ConfirmsPerFileCreate  int
	ConfirmsPerFileAppend  int
	MinerID                int
	PeerMinersAddrs        []string
	IncomingMinersAddr     string
	OutgoingMinersIP       string
	IncomingClientsAddr    string
}
type ClientHandle int
type MinerHandle int
type ClientMsg struct {
	Operation string
	FileName  string
	Content   string
}
type jsonmsg struct {
	op      string
	name    string
	content string
}

var config configSetting
var blockFile map[string]string /*创建集合 */

func readConfig(configFile string) {
	// Open our jsonFile
	jsonFile, err := os.Open(configFile)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	// println(string(readFileByte(configFile)))
	json.Unmarshal([]byte(readFileByte(configFile)), &config)
	// fmt.Println(config) // print the json setting
	defer jsonFile.Close()
}

/*
Name: readFileByte
@ para: filePath string
@ Return: string
Func: read and then return the byte of content from file in corresponding path
*/
func readFileByte(filePath string) []byte {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func printColorFont(color string, value string) {
	if color == "red" {
		fmt.Printf("%c[1;0;31m%s%c[0m\n", 0x1B, value, 0x1B) // red font
	} else if color == "blue" {
		fmt.Printf("%c[1;0;34m%s%c[0m\n", 0x1B, value, 0x1B) // blue font
	} else if color == "purple" {
		fmt.Printf("%c[1;0;35m%s%c[0m\n", 0x1B, value, 0x1B) // purple font
	} else if color == "green" {
		fmt.Printf("%c[1;0;32m%s%c[0m\n", 0x1B, value, 0x1B) // green font
	} else {
		println(value)
	}
}

func getTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (t *MinerHandle) Communication(args *ClientMsg, reply *int) error {
	*reply = 0
	println("Miner", args.Content, args.FileName, args.Operation)
	return nil
}

func listenClient() {
	ipport := strings.Split(config.IncomingClientsAddr, ":")
	ip := ipport[0]
	port, _ := strconv.Atoi(ipport[1])
	listen, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(ip), port, ""})
	if err != nil {
		fmt.Println("Fail to monitor ", err.Error())
		return
	}
	fmt.Println("client port:", config.IncomingClientsAddr)

	for {
		conn, err := listen.AcceptTCP()
		if err != nil {
			fmt.Println(getTime())
			println("Exception on client:", err.Error())
			continue
		}
		defer conn.Close()
		go func() {
			data := make([]byte, 512)
			for {
				i, err := conn.Read(data)
				if err != nil {
					break
				}
				print(getTime() + ": ")
				printColorFont("blue", "Operation: "+string(data[0:i]))

				msg := data[0:i]
				var msgjson map[string]string
				err = json.Unmarshal(msg, &msgjson)
				if err != nil {
					fmt.Println("error: ", err)
				}
				fmt.Println(msgjson["op"], msgjson["name"], msgjson["content"])

				if msgjson["op"] == "CreateFile" {
					if checkfile(msgjson["name"]) == true {
						conn.Write([]byte("FileExistsError"))
					} else {
						blockFile[msgjson["name"]] = ""
						conn.Write([]byte("success"))
					}
				} else if msgjson["op"] == "ListFiles" {
					conn.Write([]byte(getFileList()))
				} else if msgjson["op"] == "TotalRecs" {
					println("3")
				} else if msgjson["op"] == "ReadRec" {
					println("4")
				} else if msgjson["op"] == "AppendRec" {
					println("5")
				}

			}
		}()
	}
}

func listenMiner() {
	miner := new(MinerHandle)
	rpc.Register(miner)
	rpc.HandleHTTP()

	err := http.ListenAndServe(config.IncomingMinersAddr, nil)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func checkfile(fname string) bool {
	for k := range blockFile {
		if k == fname {
			return true
		}
	}
	return false
}

func getFileList() string {
	res := ""
	for fname := range blockFile {
		if len(res) == 0 {
			res += fname
		} else {
			res += ";" + fname
		}
	}
	if len(res) == 0 {
		res = "No book"
	}
	return res
}

func showfiles() {
	println("------------------------")
	for k, v := range blockFile {
		fmt.Printf("%s: %s\n", k, v)
	}
	println("------------------------")
}

func init() {
	blockFile = make(map[string]string)
}

func main() {
	if len(os.Args) != 2 {
		println("go run miner.go [settings]")
		return
	}
	readConfig(os.Args[1])

	go listenMiner()
	go listenClient()

	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		if text == "showfiles\n" {
			showfiles()
		}
	}
}
