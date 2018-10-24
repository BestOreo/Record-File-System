package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
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
	"sync"
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
	MinerID                string
	PeerMinersAddrs        []string
	IncomingMinersAddr     string
	OutgoingMinersIP       string
	IncomingClientsAddr    string
}
type ClientHandle int
type MinerHandle int
type ClientMsg struct {
	Operation string
	MinerID   string
}
type OpMsg struct {
	MinerID string
	MsgID   uint
	Op      string
	Name    string
	Content string
}
type Record [512]byte

var config configSetting
var globalMsgID uint
var blockFile map[string]string /*创建集合 */
var recordQueue []OpMsg

var minerChain *BlockChain

// type filenode struct {
// 	count   int
// 	content [65535]Record
// }

// var blockFile map[string]filenode

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
		res = "No File"
	}
	return res
}

func showfiles() {
	println("------------------------")
	for k, v := range blockFile {
		fmt.Printf("Name:%s  Length:%d\n", k, len(v))
		for i := 0; i*512 < len(v); i++ {
			fmt.Printf("%d. %s\n", i, []byte(v)[i*512:i*512+512])
		}
	}
	println("------------------------")
}

func pushRecordQueue(opmsg OpMsg) {
	recordQueue = append(recordQueue, opmsg)
}

func popRecordQueue() OpMsg {
	head := recordQueue[0]
	recordQueue = recordQueue[1:]
	return head
}

func printRecordQueue() {
	println("---------------------------")
	for i := 0; i < len(recordQueue); i++ {
		println(recordQueue[i].MinerID, recordQueue[i].MsgID, recordQueue[i].Op)
	}
	println("---------------------------")
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

func (t *MinerHandle) MinerTalk(args *ClientMsg, reply *int) error {
	*reply = 0
	println(args.Operation, "from", args.MinerID)
	return nil
}

func checkOperationInQueue(msg *OpMsg) bool {
	for i := 0; i < len(recordQueue); i++ {
		if recordQueue[i].MinerID == msg.MinerID && recordQueue[i].MsgID == msg.MsgID {
			return true
		}
	}
	return false
}

func (t *MinerHandle) ShareOperation(record *OpMsg, reply *int) error {
	*reply = 0
	println("------------")
	println("| Msg: ", record.MinerID, record.MsgID, record.Op, record.Name, record.Content)
	println("------------")
	if checkOperationInQueue(record) == false {
		printColorFont("green", "pushed into queue")
		pushRecordQueue(*record)
		broadcastOperations(*record)
	}
	return nil
}

func listenMiner() {
	miner := new(MinerHandle)
	rpc.Register(miner)
	rpc.HandleHTTP()
	minerChain = &BlockChain{
		chainLock:    &sync.Mutex{},
		chain:        make([]*Block, 0),
		txBuffer:     make([]*Tx, 0),
		txBufferSize: 10,
		difficulty:   2,
	}
	minerChain.init()
	err := http.ListenAndServe(config.IncomingMinersAddr, nil)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func generateOpMsg(op string, name string, content string) OpMsg {
	operationMsg := OpMsg{config.MinerID, globalMsgID, op, name, content}
	globalMsgID++
	return operationMsg
}

func sendMiner(remoteIPPort string, args ClientMsg) {
	client, err := rpc.DialHTTP("tcp", remoteIPPort)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	// Synchronous call
	var reply int
	err = client.Call("MinerHandle.MinerTalk", args, &reply)
	if err != nil {
		log.Fatal("tcp error:", err)
	}
	if reply == 0 {
		fmt.Printf("%s: send to %s successfully\n", getTime(), remoteIPPort)
	}
}

func broadcastOperations(operationMsg OpMsg) {
	for _, ip := range config.PeerMinersAddrs {
		client, err := rpc.DialHTTP("tcp", ip)
		if err != nil {
			log.Fatal("dialing:", err)
		}
		var reply int
		err = client.Call("MinerHandle.ShareOperation", operationMsg, &reply)
		if err != nil {
			log.Fatal("tcp error:", err)
		}
		if reply == 0 {
			fmt.Printf("%s: send to %s successfully\n", getTime(), remoteIPPort)
		}
	}
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
	// fmt.Println("client port:", config.IncomingClientsAddr)

	for {
		conn, err := listen.AcceptTCP()
		if err != nil {
			fmt.Println(getTime())
			println("Exception on client:", err.Error())
			continue
		}
		defer conn.Close()
		go func() {
			data := make([]byte, 1024) // data received by tcp
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

				// Client CreateFile
				if msgjson["op"] == "CreateFile" {
					if checkfile(msgjson["name"]) == true {
						conn.Write([]byte("FileExistsError"))
					} else {
						blockFile[msgjson["name"]] = "" // create a new files
						fmt.Println("-----------------")
						fmt.Println(msgjson)
						fmt.Println("-----------------")
						minerChain.createTransaction(msgjson["op"], msgjson["name"], msgjson["content"])
						// codes about blockchain
						conn.Write([]byte("success"))
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["content"])
						pushRecordQueue(operationMsg)
						broadcastOperations(operationMsg)
					}
					// Client ListFiles
				} else if msgjson["op"] == "ListFiles" {
					conn.Write([]byte(getFileList()))
					// Client TotalRecs
				} else if msgjson["op"] == "TotalRecs" {
					if checkfile(msgjson["name"]) == false {
						conn.Write([]byte("FileDoesNotExistError"))
					} else {
						conn.Write([]byte(strconv.Itoa(len(blockFile[msgjson["name"]]) / 512)))
					}
					// Client ReadRec
				} else if msgjson["op"] == "ReadRec" {
					pos, err := strconv.Atoi(msgjson["content"])
					if err != nil {
						log.Fatal("record num isn't integer")
						continue
					}
					if checkfile(msgjson["name"]) == false {
						conn.Write([]byte("FileDoesNotExistError"))
					} else if len(blockFile[msgjson["name"]])/512-1 < pos {
						conn.Write([]byte("RecordDoesNotExistError"))
					} else {
						conn.Write([]byte(blockFile[msgjson["name"]][pos*512 : pos*512+512]))
					}
					// Client AppendRec
				} else if msgjson["op"] == "AppendRec" {
					if checkfile(msgjson["name"]) == false {
						conn.Write([]byte("FileDoesNotExistError"))
					} else if len(msgjson["content"])/512 > 655354 { // have at most 65,5354 (uint16) records
						conn.Write([]byte("FileMaxLenReachedError"))
					} else {
						var m [512]byte
						copy(m[:], []byte(msgjson["content"]))
						blockFile[msgjson["name"]] += string(m[:])

						// codes about blockchain

						conn.Write([]byte(strconv.Itoa(len(blockFile[msgjson["name"]])/512 - 1)))
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["content"])
						pushRecordQueue(operationMsg)
						broadcastOperations(operationMsg)
					}
				}
			}
		}()
	}
}

/*** Blockchain ***/

// TX_BUFFER_SIZE is the maximum number of tx's from clients before a new block is created
var TX_BUFFER_SIZE = 1

// Tx represents a single transaction from a client
type Tx struct {
	opType  string
	content string
	minerID string
}

// Block is a single structure in the chain
type Block struct {
	prevHash     string
	nonce        uint32
	transactions []*Tx
}

// BlockChain is the central datastructure
type BlockChain struct {
	chainLock    *sync.Mutex
	chain        []*Block
	txBuffer     []*Tx
	txBufferSize int
	difficulty   int
}

func (bc *BlockChain) init() {
	// create genesis block
	block := &Block{}
	block.prevHash = ""
	block.nonce = 0
	bc.addBlockToChain(block)
	fmt.Println("Genisis block created.")
}

func (bc *BlockChain) addBlockToChain(block *Block) {
	// add it to the chain
	bc.chainLock.Lock()
	bc.chain = append(bc.chain, block)
	bc.chainLock.Unlock()

	fmt.Println("Block Added")
}

func (bc *BlockChain) createBlock() {
	// set prev hash
	block := &Block{}
	block.prevHash = bc.hashBlock(bc.chain[len(bc.chain)-1])
	block.transactions = bc.txBuffer

	// mine the block to find solution
	block.nonce = bc.proofOfWork(block)
	bc.addBlockToChain(block)
	bc.txBuffer = make([]*Tx, 0)
	bc.printChain()
}
func (bc *BlockChain) proofOfWork(block *Block) (nonce uint32) {
	nonce = block.nonce
	str := bc.hashBlock(block)
	for {
		foundSolution := strings.HasSuffix(str, "00")
		if foundSolution {
			break
		}
		nonce++
		block.nonce = nonce
		str = bc.hashBlock(block)
	}
	fmt.Println(str)
	return nonce
}

func (bc *BlockChain) hashBlock(block *Block) (str string) {
	hash := md5.New()
	blockData := bc.getBlockBytes(block)
	hash.Write(blockData)
	str = hex.EncodeToString(hash.Sum(nil))
	return str
}
func (bc *BlockChain) createTransaction(opType string, fileName string, minerID string) {
	currBuff := len(bc.txBuffer)
	fmt.Println(currBuff)
	if currBuff <= TX_BUFFER_SIZE {
		bc.chainLock.Lock()
		tx := &Tx{
			opType,
			fileName,
			minerID,
		}
		bc.txBuffer = append(bc.txBuffer, tx)
		bc.chainLock.Unlock()
	} else {
		bc.createBlock()
	}
}

func (bc *BlockChain) getBlockBytes(block *Block) []byte {
	txString := ""
	for _, tx := range block.transactions {
		txString = tx.content + tx.opType + tx.minerID
	}
	// timeBytes, err := time.Now().MarshalJSON()
	// if err != nil {
	// 	panic("time failed")
	// }
	nonceStr := fmt.Sprint(block.nonce)
	data := bytes.Join(
		[][]byte{
			[]byte(block.prevHash),
			[]byte(txString),
			// timeBytes,
			[]byte(nonceStr),
		},
		[]byte{},
	)
	return data
}

func (bc *BlockChain) printChain() {
	for idx, block := range bc.chain {
		fmt.Println("****************************")
		fmt.Println("Block Number: ")
		fmt.Println(idx)
		fmt.Println("Prev Hash: ")
		fmt.Println(block.prevHash)
		fmt.Println("Transactions: ")
		for _, tx := range block.transactions {
			fmt.Println(".............")
			fmt.Println("OP Type: ")
			fmt.Println(tx.opType)
			fmt.Println("Content: ")
			fmt.Println(tx.content)
			fmt.Println("MinerId: ")
			fmt.Println(tx.minerID)
			fmt.Println(".............")
		}
		fmt.Println("****************************")
	}
}

func Initial() {
	blockFile = make(map[string]string)
	recordQueue = make([]OpMsg, 0)
}

/*** END Blockchain ***/

func main() {
	if len(os.Args) != 2 {
		println("go run miner.go [settings]")
		return
	}
	readConfig(os.Args[1]) // read the config.json into var config configSetting
	fmt.Printf("MinerID:%s\nclientPort:%s\nPeerMinersAddrs:%v\nIncomingMinersAddr:%s\n", config.MinerID, config.IncomingClientsAddr, config.PeerMinersAddrs, config.IncomingMinersAddr)
	Initial()

	go listenMiner()  // Open a port to listen msg from miners
	go listenClient() // Open a port to listen msg from clients

	// command line control
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		if text == "showfiles\n" {
			showfiles()
		} else if strings.Contains(text, "miner") == true {
			// demo : inform neiboring peers
			for _, ip := range config.PeerMinersAddrs {
				sendMiner(ip, ClientMsg{config.MinerID + " says hello ", config.MinerID})
			}
		} else if strings.Contains(text, "queue") == true {
			printRecordQueue()
		} else if strings.Contains(text, "pop") == true {
			popRecordQueue()
		}
	}
}
