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
	"math/rand"
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
var recordQueue []*OpMsg
var recordQueueMutex sync.Mutex
var recordTrash []*OpMsg
var recordTrashMutex sync.Mutex

var blockQueue []*Block
var blockQueueMutex sync.Mutex

var minerChain *BlockChain

// type filenode struct {
// 	count   int
// 	Content [65535]Record
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
Func: read and then return the byte of Content from file in corresponding path
*/
func readFileByte(filePath string) []byte {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func checkfile(fname string) bool {
	longestMutex.Lock()
	lastblock := longestChainNodes[rand.Int()%len(longestChainNodes)]
	longestMutex.Unlock()

	for {
		if lastblock == nil {
			return false
		}
		if strings.Contains(lastblock.block.Transactions, "CreateFile"+"{,}"+fname) == true {
			return true
		}
		lastblock = lastblock.parent
	}
	return false
}

func getAllRecordByName(fname string) []string {
	longestMutex.Lock()
	lastblock := longestChainNodes[rand.Int()%len(longestChainNodes)]
	longestMutex.Unlock()
	res := make([]string, 0)

	for i := 0; i < config.ConfirmsPerFileAppend; i++ {
		if lastblock == nil {
			return res
		}
		lastblock = lastblock.parent
	}

	for {
		if lastblock == nil {
			return res
		}
		if strings.Contains(lastblock.block.Transactions, "AppendRec"+"{,}"+fname) == true {
			jsons := convertJsonArray(lastblock.block.Transactions)
			for _, json := range jsons {
				if json["op"] == "AppendRec" && json["filename"] == fname {
					res = append(res, json["content"])
				}
			}
		}
		lastblock = lastblock.parent
	}
	return res
}

func getFileList() string {
	res := ""

	longestMutex.Lock()
	lastblock := longestChainNodes[rand.Int()%len(longestChainNodes)]
	longestMutex.Unlock()

	for {
		if lastblock == nil {
			break
		}
		if strings.Contains(lastblock.block.Transactions, "CreateFile") == true {
			jsons := convertJsonArray(lastblock.block.Transactions)
			for _, json := range jsons {
				if json["op"] == "CreateFile" {
					if len(res) == 0 {
						res += json["filename"]
					} else {
						res += ";" + json["filename"]
					}
				}
			}
		}
		lastblock = lastblock.parent
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

func hashOpMsg(opmsg *OpMsg) (hashStr string) {
	recordQueueMutex.Lock()
	str := opmsg.Content + opmsg.MinerID + opmsg.Name + opmsg.Op
	hash := md5.New()
	hash.Write([]byte(str))
	hashStr = hex.EncodeToString(hash.Sum(nil))
	return hashStr
}

func checkOpHashMap(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

var OpHashMap []string

/*******************************************/
// Queue Opearation
// func pushRecordQueue(opmsg *OpMsg) {
// 	recordQueueMutex.Lock()
// 	hashOp := hashOpMsg(opmsg)
// 	if checkOpHashMap(hashOp, OpHashMap) {
// 		fmt.Println("Has transaction")
// 	} else {
// 		recordQueue = append(recordQueue, opmsg)
// 		OpHashMap = append(OpHashMap, hashOp)
// 	}
// 	recordQueueMutex.Unlock()
// }

func pushRecordQueue(opmsg *OpMsg) {
	for _, q := range recordQueue {
		if q.Op == "CreateFile" && opmsg.Op == "CreateFile" && q.Name == opmsg.Name {
			return
		}
	}
	recordQueueMutex.Lock()
	recordQueue = append(recordQueue, opmsg)
	recordQueueMutex.Unlock()
}

func popRecordQueue() *OpMsg {
	if len(recordQueue) == 0 {
		return nil
	}
	head := recordQueue[0]
	recordQueueMutex.Lock()
	recordQueue = recordQueue[1:]
	recordQueueMutex.Unlock()
	recordTrashMutex.Lock()
	recordTrash = append(recordTrash, head)
	recordTrashMutex.Unlock()
	return head
}

func pushBlockQueue(block *Block) {
	blockQueueMutex.Lock()
	blockQueue = append(blockQueue, block)
	blockQueueMutex.Unlock()
}

func popBlockQueue() *Block {
	if len(blockQueue) == 0 {
		return nil
	}
	head := blockQueue[0]
	blockQueue = blockQueue[1:]
	return head
}

// In order to avoid message flooding in loop network
func checkOperationInQueue(msg *OpMsg) bool {
	for i := 0; i < len(recordQueue); i++ {
		if recordQueue[i].MinerID == msg.MinerID && recordQueue[i].MsgID == msg.MsgID {
			return true
		}
	}
	for i := 0; i < len(recordTrash); i++ {
		if recordTrash[i].MinerID == msg.MinerID && recordTrash[i].MsgID == msg.MsgID {
			return true
		}
	}
	return false
}

// In order to avoid message flooding in loop network
func checkBlockInQueue(block *Block) bool {
	for i := 0; i < len(blockQueue); i++ {
		if blockQueue[i].Nonce == block.Nonce && blockQueue[i].PrevHash == block.PrevHash {
			return true
		}
	}
	return false
}

func printRecordQueue() {
	println("---------------------------\nRecordQueue")
	for i := 0; i < len(recordQueue); i++ {
		println(recordQueue[i].MinerID, recordQueue[i].Op, recordQueue[i].Name, recordQueue[i].Content)
	}
	println("---------------------------")
}

func printBlockQueue() {
	println("---------------------------\nBlockQueue")
	for i := 0; i < len(blockQueue); i++ {
		println("***", i)
		block := blockQueue[i]
		println("Repeated: False")
		println("pre-Hash:", block.PrevHash)
		println("index", block.Index)
		println("MinerID", block.Miner)
		println("Transactions:")
		jsons := convertJsonArray(block.Transactions)
		for i := 0; i < len(jsons); i++ {
			json := jsons[i]
			fmt.Printf("%d\top:%s\tfilename:%s\tcontent:%s\n", i, json["op"], json["filename"], json["content"])
		}
		println("timestamp", block.Timestamp)
	}
	println("---------------------------")
}

/*******************************************/

// print message in assigned color
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

// show time
func getTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func makeTimestamp() int64 {
	return int64(time.Now().UnixNano())
}

// just a demo
func (t *MinerHandle) MinerTalk(args *ClientMsg, reply *int) error {
	*reply = 0
	println(args.Operation, "from", args.MinerID)
	return nil
}

// thread to deal with message from nearby miners
func listenMiner() {
	miner := new(MinerHandle)
	rpc.Register(miner)
	rpc.HandleHTTP()
	err := http.ListenAndServe(config.IncomingMinersAddr, nil)
	if err != nil {
		fmt.Println(err.Error())
	}
}

// generate a opeation message struct
func generateOpMsg(op string, name string, Content string) OpMsg {
	operationMsg := OpMsg{config.MinerID, globalMsgID, op, name, Content}
	globalMsgID++
	return operationMsg
}

// just a demo
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
		fmt.Printf("*** send %v to %s successfully\n", args, remoteIPPort)
	}
}

/******************************************/
// RPC Handler

// FloodOperation : flood operation of client to the whole network
func (t *MinerHandle) FloodOperation(record *OpMsg, reply *int) error {
	*reply = 0
	if checkOperationInQueue(record) == false {
		println("------------")
		println("| Record: ", record.MinerID, record.MsgID, record.Op, record.Name, record.Content)
		printColorFont("green", "| pushed into recordQueue")
		println("------------")
		pushRecordQueue(record)
		// minerChain.createTransaction(record.Op, record.Name, record.Content, record.MinerID)
		broadcastOperations(*record)
	}
	return nil
}

// FloodBlock : flood block to the whole network
func (t *MinerHandle) FloodBlock(block *Block, reply *int) error {
	*reply = 0
	println(getTime())
	printColorFont("purple", "*** Receive Block")

	if checkBlockInQueue(block) == false {
		if minerChain.verifyBlock(block) == false {
			return nil
		}
		println("------------")
		println("Repeated: False")
		println("pre-Hash:", block.PrevHash)
		println("index:", block.Index)
		println("MinerID:", block.Miner)
		// println("tx", block.Transactions)
		println("Transactions:")
		if block.Transactions != "" {
			jsons := convertJsonArray(block.Transactions)
			for i := 0; i < len(jsons); i++ {
				json := jsons[i]
				fmt.Printf("%d\top:%s\tfilename:%s\tcontent:%s\n", i, json["op"], json["filename"], json["content"])
			}
		}
		println("timestamp:", block.Timestamp)
		println("------------")
		println()
		pushBlockQueue(block)

		// minerChain.addBlockToChain(block) //old way

		root.addChild(*block) // new way

		broadcastBlocks(block)

	} else {
		println("Repeated: True")
		println("From", block.Miner)
		printColorFont("purple", "***")
	}
	println()
	return nil
}

/******************************************/

// broadcast opearation of client to whole network
func broadcastOperations(operationMsg OpMsg) {
	for _, ip := range config.PeerMinersAddrs {
		client, err := rpc.DialHTTP("tcp", ip)
		if err != nil {
			println("dialing:", err)
			continue
		}
		var reply int
		err = client.Call("MinerHandle.FloodOperation", operationMsg, &reply)
		if err != nil {
			println("tcp error:", err)
			continue
		}
		if reply == 0 {
			fmt.Printf("!!! send %v to %s successfully\n", operationMsg, ip)
		}
	}
}

// broadcastBlocks broadcast blcok to whole network
func broadcastBlocks(block *Block) {
	println(getTime())
	fmt.Println("---------BROADCAST--------")
	println("Index:", block.Index)
	println("Transactions:", block.Transactions)
	println("Timestamp:", block.Timestamp)
	for _, ip := range config.PeerMinersAddrs {
		client, err := rpc.DialHTTP("tcp", ip)
		if err != nil {
			println("dialing:", err)
			continue
		}
		var reply int
		err = client.Call("MinerHandle.FloodBlock", block, &reply)
		if err != nil {
			println("tcp error:", err)
			continue
		}
		if reply == 0 {
			fmt.Printf("send block to %s successfully\n", ip)
		}
	}
	fmt.Println("---------End BROADCAST--------")
	println()
}

// thread to deal with incoming message from client
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
						// codes about blockchain
						// conn.Write([]byte("success"))
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["content"])
						conn.Write([]byte(strconv.Itoa(int(operationMsg.MsgID)) + ";" + strconv.Itoa(config.GenOpBlockTimeout) + ";" + config.MinerID))
						pushRecordQueue(&operationMsg)
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
						records := getAllRecordByName(msgjson["name"])
						conn.Write([]byte(strconv.Itoa(len(records))))
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
						continue
					}
					records := getAllRecordByName(msgjson["name"])
					if len(records) < pos {
						conn.Write([]byte("RecordDoesNotExistError"))
					} else {
						conn.Write([]byte(records[pos]))
					}
					// Client AppendRec
				} else if msgjson["op"] == "AppendRec" {
					if checkfile(msgjson["name"]) == false {
						conn.Write([]byte("FileDoesNotExistError"))
						continue
					}
					records := getAllRecordByName(msgjson["name"])
					if len(records) >= 65535 { // have at most 65,5354 (uint16) records
						conn.Write([]byte("FileMaxLenReachedError"))
					} else {
						// codes about blockchain
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["content"])
						// retrun msgID,time interval, ConfirmsPerFileAppend, current length
						longestMutex.Lock()
						conn.Write([]byte(strconv.Itoa(int(operationMsg.MsgID)) + ";" + strconv.Itoa(config.GenOpBlockTimeout) + ";" + config.MinerID))
						longestMutex.Unlock()
						pushRecordQueue(&operationMsg)
						broadcastOperations(operationMsg)
					}
				} else if msgjson["op"] == "queryFile" {
					tranction := msgjson["name"]
					minerID := msgjson["content"]
					reply := queryFilePos(tranction, minerID)
					conn.Write([]byte(reply))
				}
			}
		}()
	}
}

func queryFilePos(transaction string, minerID string) string {
	longestMutex.Lock()
	lastblock := longestChainNodes[rand.Int()%len(longestChainNodes)] // pick the longest chain randomly
	curLength := maxLength
	longestMutex.Unlock()

	for {
		if lastblock == nil {
			return "wait"
		}
		if strings.Contains(lastblock.block.Transactions, transaction) == true {
			if strings.Contains(lastblock.block.Transactions, minerID) == false {
				return "false"
			}
			if curLength-lastblock.block.Index >= config.ConfirmsPerFileCreate {
				return "true"
			}
		}
		lastblock = lastblock.parent
	}
}

/*** Blockchain ***/
type BlockNode struct {
	block         Block
	hashvalue     string
	blockChildren []*BlockNode
	parent        *BlockNode
}

var root BlockNode

var longestMutex sync.Mutex        // longestMutex to protect both maxLength and longestChainNodes
var maxLength int                  // length of longest chain
var longestChainNodes []*BlockNode // to record the tail node address of longest chain

func (root *BlockNode) addChild(node Block) {
	println("-------- addChild ---------")
	parent := root.findNode(node.PrevHash)
	if parent == nil {
		printColorFont("red", "No such node has prevHash: "+node.PrevHash)
	} else {
		println("Add into tree successfully")
		child := &BlockNode{node, minerChain.hashBlock(&node), nil, parent}
		parent.blockChildren = append(parent.blockChildren, child)

		// find a new longest chain
		if node.Index > maxLength {
			longestMutex.Lock()
			maxLength = node.Index
			longestChainNodes = make([]*BlockNode, 0)
			longestChainNodes = append(longestChainNodes, child)
			longestMutex.Unlock()
		} else if node.Index == maxLength {
			// more than one longest chain
			longestMutex.Lock()
			longestChainNodes = append(longestChainNodes, child)
			longestMutex.Unlock()
		}

	}
	println("-------- End addChild ---------\n")

}

func (parent *BlockNode) findNode(prehash string) *BlockNode {
	if prehash == parent.hashvalue {
		return parent
	}
	for _, node := range parent.blockChildren {
		n := node.findNode(prehash)
		if n != nil {
			return n
		}
	}
	return nil
}

func printTreeNode(node BlockNode, suffix string) {
	println(suffix+"Index:", node.block.Index)
	println(suffix+"Hashvalue:", node.hashvalue)
	println(suffix+"PrevHash:", node.block.PrevHash)
	println(suffix+"Timestamp:", node.block.Timestamp)
	println(suffix+"Miner:", node.block.Miner)
	println(suffix+"Transactions:", node.block.Transactions)
	println(suffix+"Nonce:", node.block.Nonce)
	println(suffix + "Children:")
	for i := 0; i < len(node.blockChildren); i++ {
		child := node.blockChildren[i]
		println(suffix + "\t" + strconv.Itoa(i))
		printTreeNode(*child, suffix+"\t")
		println()
	}
}

func (root *BlockNode) printTree() {
	println("---------------------- Tree ------------------------------")
	printTreeNode(*root, "")
	println("---------------------- End Tree --------------------------")
}

// Block is a single structure in the chain
type Block struct {
	PrevHash     string
	Index        int   // the position in blockchain
	Timestamp    int64 // nanoseconds elapsed since January 1, 1970 UTC.
	Nonce        uint32
	Miner        string
	Transactions string
}

// BlockChain is the central datastructure
type BlockChain struct {
	chainLock    *sync.Mutex
	chain        []*Block
	maxRecordNum int
	difficulty   int
}

func (bc *BlockChain) init() {
	OpHashMap = make([]string, 0)
	// create genesis block
	block := &Block{}
	block.PrevHash = config.GenesisBlockHash
	block.Nonce = 0
	bc.chain = append(bc.chain, block)
	fmt.Println("Genisis block created.")

	// tree
	root = BlockNode{*block, minerChain.hashBlock(block), nil, nil} // initial tree

	longestMutex.Lock()
	longestChainNodes = append(longestChainNodes, &root)
	maxLength = 0
	longestMutex.Unlock()

	bc.startBlockGeneration()

	//bc.manageChain()
}

func (bc *BlockChain) startBlockGeneration() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			bc.createTransactionBlock()
		}
	}()
}

func (bc *BlockChain) manageChain() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for t := range ticker.C {
			fmt.Println(t)
			bc.difficulty = len(bc.chain) / 2
			// perform any
		}
	}()
}

// This function should only occur when the chain is locked.
func (bc *BlockChain) verifyBlock(block *Block) (isValidBlock bool) {
	numberOfZeros := strings.Repeat("0", bc.difficulty)
	blockHash := bc.hashBlock(block)
	parent := root.findNode(block.PrevHash)
	if parent == nil {
		return false
	}
	hasCorrectHash := strings.HasSuffix(blockHash, numberOfZeros)
	if !hasCorrectHash {
		fmt.Println("BlockHash:\t", blockHash)
		fmt.Println("BlockNonce:\t", block.Nonce)
		fmt.Println("Hint: Incorrect hash")
		return false
	}
	return true
}

func (bc *BlockChain) validateCoinBalance(minerID string) (valid bool) {
	return true
}

func checkRecordInChain(record *OpMsg, node *BlockNode) bool {
	var str string
	if record.Op == "CreateFile" {
		str = record.Op + "{,}" + record.Name
	} else {
		str = record.Op + "{,}" + record.Name + "{,}" + record.Content + "{,}" + record.MinerID + "{,}" + strconv.Itoa(int(record.MsgID))
	}
	for {
		if node.parent == nil {
			return false
		}
		if strings.Contains(node.block.Transactions, str) == true {
			return true
		}
		node = node.parent
	}
}

// when timeout, you just use API createBlock
// the function will read records from the queue
// and generate a new block
func (bc *BlockChain) createTransactionBlock() {
	// set prev hash
	block := &Block{}

	longestMutex.Lock()
	lastblock := longestChainNodes[rand.Int()%len(longestChainNodes)] // pick the longest chain randomly
	block.Index = lastblock.block.Index + 1
	longestMutex.Unlock()

	block.PrevHash = lastblock.hashvalue

	// block.Transactions = bc.txBuffer
	block.Timestamp = makeTimestamp()
	block.Miner = config.MinerID

	var transactionNum int

	for {
		if len(recordQueue) == 0 {
			break //note
		}
		record := popRecordQueue()
		if checkRecordInChain(record, lastblock) == true {
			str := record.Op + "{,}" + record.Name + "{,}" + record.Content + "{,}" + record.MinerID + "{,}" + strconv.Itoa(int(record.MsgID))
			printColorFont("red", config.MinerID+" "+str+" "+lastblock.block.Transactions)
			continue
		} else {
			if len(block.Transactions) == 0 {
				block.Transactions = record.Op + "{,}" + record.Name + "{,}" + record.Content + "{,}" + record.MinerID + "{,}" + strconv.Itoa(int(record.MsgID))
			} else {
				block.Transactions += "{;}" + record.Op + "{,}" + record.Name + "{,}" + record.Content + "{,}" + record.MinerID + "{,}" + strconv.Itoa(int(record.MsgID))
			}
			transactionNum++
		}
		if transactionNum == bc.maxRecordNum {
			break
		}
	}

	// it's not right, just for convenience
	if transactionNum == 0 {
		if disableNoOp == true {
			return
		}
	}

	// mine the block to find solution
	block.Nonce = bc.proofOfWork(block)

	println("*****************")
	fmt.Printf("* Block Info\n")
	println("*Index:", block.Index)
	println("*Miner:", block.Miner)
	println("*Transaction:", block.Transactions)
	println("*Nonce:", block.Nonce)
	println("*****************")

	broadcastBlocks(block)
}

func (bc *BlockChain) proofOfWork(block *Block) (Nonce uint32) {
	println("-----------ProofOfWork-----------")
	Nonce = block.Nonce
	str := bc.hashBlock(block)
	for {
		numberOfZeros := strings.Repeat("0", bc.difficulty)
		foundSolution := strings.HasSuffix(str, numberOfZeros)
		if foundSolution {
			break
		}
		Nonce++
		block.Nonce = Nonce
		str = bc.hashBlock(block)
	}
	fmt.Println("Found Solution: (prevhash/currhash/nonce) ")
	fmt.Println("PrevHash", block.PrevHash)
	fmt.Println("currhash", str)
	fmt.Println("Nonce:", Nonce)
	println("-----------End ProofOfWork-----------")
	println()
	return Nonce
}

func (bc *BlockChain) hashBlock(block *Block) (str string) {
	hash := md5.New()
	blockData := bc.getBlockBytes(block)
	hash.Write(blockData)
	str = hex.EncodeToString(hash.Sum(nil))
	return str
}

func (bc *BlockChain) getBlockBytes(block *Block) []byte {
	data := bytes.Join(
		[][]byte{
			[]byte(block.PrevHash),
			[]byte(fmt.Sprint(strconv.Itoa(block.Index))),
			[]byte(strconv.FormatInt(block.Timestamp, 10)),
			[]byte(fmt.Sprint(block.Nonce)),
			[]byte(block.Transactions),
		},
		[]byte{},
	)
	return data
}

func convertJsonArray(transaction string) []map[string]string {
	res := make([]map[string]string, 0)
	if len(transaction) == 0 {
		return res
	}
	recordList := strings.Split(transaction, "{;}")
	for _, record := range recordList {
		elements := strings.Split(record, "{,}")
		json := make(map[string]string)
		json["op"] = elements[0]
		if json["op"] == "No-Op" {
			json["filename"] = ""
			json["content"] = ""
			json["minerId"] = elements[3]
			json["msgid"] = elements[4]
		} else {
			json["filename"] = elements[1]
			json["content"] = elements[2]
			json["minerId"] = elements[3]
			json["msgid"] = elements[4]
		}
		res = append(res, json)
	}
	return res
}

// maps blockprevhash -> index of content in the block that have the filename
func (bc *BlockChain) getFileNames() (fileNames []string) {
	fileNames = make([]string, 0)
	for _, block := range bc.chain {
		jsons := convertJsonArray(block.Transactions)
		for i := 0; i < len(jsons); i++ {
			json := jsons[i]
			if json["op"] == "CreateFile" {
				fileNames = append(fileNames, json["filename"])
			}
		}
	}
	return fileNames
}

func (bc *BlockChain) validateTransactions(fileName string, opType string, content string) (valid bool) {
	for _, block := range bc.chain {
		jsons := convertJsonArray(block.Transactions)
		for i := 0; i < len(jsons); i++ {
			json := jsons[i]
			if json["op"] == opType && json["filename"] == fileName && json["content"] == content && json["op"] != "AppendRec" && json["op"] != "No-Op" {
				return false
			}
		}
	}
	return true
}

// func (bc *BlockChain) findFiles(fileName string) {
// 	blockTxMap := make(map[string][]int)
// 	for _, block := range bc.chain {
// 		for txID, tx := range block.Transactions {
// 			hasBlock := false
// 			if tx.filename == fileName {
// 				if !hasBlock {
// 					blockTxMap[block.PrevHash] = make([]int, 0)
// 					hasBlock = true
// 				}
// 				blockTxMap[block.PrevHash] = append(blockTxMap[block.PrevHash], txID)
// 				fmt.Println(block.PrevHash)
// 				fmt.Println(tx.filename)
// 			}
// 		}
// 	}
// 	fmt.Println("map:", blockTxMap)
// }

func (bc *BlockChain) printChain() {
	for i, block := range bc.chain {
		fmt.Println("****************************")
		fmt.Println("Block Index: ", block.Index)
		fmt.Println("Prev Hash: ", block.PrevHash)
		println("Transactions:")
		if i != 0 && block.Transactions != "" {
			jsons := convertJsonArray(block.Transactions)
			for i := 0; i < len(jsons); i++ {
				json := jsons[i]
				fmt.Printf("%d\top:%s\tfilename:%s\tcontent:%s\n", i, json["op"], json["filename"], json["content"])
			}
		}
		fmt.Println("Nonce: ", block.Nonce)
		fmt.Println("MinerID: ", block.Miner)
		fmt.Println("Timestamp: ", block.Timestamp)
		fmt.Println("****************************")
		println()
	}
}

func Initial() {
	blockFile = make(map[string]string)
	recordQueue = make([]*OpMsg, 0)
	recordTrash = make([]*OpMsg, 0)
	blockQueue = make([]*Block, 0)
	minerChain = &BlockChain{
		chainLock:    &sync.Mutex{},
		chain:        make([]*Block, 0),
		maxRecordNum: 2, // the maximum records in one block
		difficulty:   5,
	}
	minerChain.init()

	rand.Seed(time.Now().Unix())
}

/*** END Blockchain ***/

var disableNoOp bool

func main() {

	disableNoOp = true

	if len(os.Args) != 2 {
		println("go run miner.go [settings]")
		return
	}
	readConfig(os.Args[1]) // read the config.json into var config configSetting
	fmt.Printf("MinerID:%s\nclientPort:%s\nPeerMinersAddrs:%v\nIncomingMinersAddr:%s\n", config.MinerID, config.IncomingClientsAddr, config.PeerMinersAddrs, config.IncomingMinersAddr)
	Initial()

	go listenMiner()  // Open a port to listen msg from miners
	go listenClient() // Open a port to listen msg from clients
	// go minerChain.printChain()

	// command line control
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		if text == "showfiles\n" {
			showfiles()
		} else if strings.Contains(text, "rpcdemo") == true {
			// demo : inform neiboring peers
			for _, ip := range config.PeerMinersAddrs {
				sendMiner(ip, ClientMsg{config.MinerID + " says hello ", config.MinerID})
			}
		} else if strings.Contains(text, "rqueue") == true {
			printRecordQueue()
		} else if strings.Contains(text, "bqueue") == true {
			printBlockQueue()
		} else if strings.Contains(text, "lfiles") == true {
			fileNames := minerChain.getFileNames()
			for _, name := range fileNames {
				fmt.Println(name)
			}
		} else if strings.Contains(text, "pop") == true {
			rec := popRecordQueue()
			if rec == nil {
				println("The queue is empty")
			}
		} else if strings.Contains(text, "floodblock") == true {
			broadcastBlocks(&Block{"Hello", 0, 0, 65535, "Miner", "A,B,C,D"})
		} else if strings.Contains(text, "createblock") == true {
			minerChain.createTransactionBlock()
		} else if strings.Contains(text, "chain") == true {
			minerChain.printChain()
		} else if strings.Contains(text, "test") == true {
			s := convertJsonArray("CreateFile{,}text.txt{,}{,}Mijnwerker{;}CreateFile{,}text2.txt{,}{,}Mijnwerker")
			for _, v := range s {
				println(v["op"], v["filename"], v["content"])
			}
		} else if strings.Contains(text, "treetest") == true {
			b := Block{root.hashvalue, 0, 0, 65535, "Miner", "A,B,C,D"}
			b1 := Block{minerChain.hashBlock(&b), 0, 0, 1234, "ad", "C,D"}
			b2 := Block{minerChain.hashBlock(&b), 0, 0, 1234, "ad", "C,D"}
			c := Block{root.hashvalue, 0, 0, 1234, "ad", "C,D"}
			d := Block{minerChain.hashBlock(&c), 0, 0, 1234, "ad", "C,D"}
			root.addChild(b)
			root.addChild(b1)
			root.addChild(b2)
			root.addChild(c)
			root.addChild(d)
			root.printTree()
		} else if strings.Contains(text, "tree") == true {
			root.printTree()
		} else if strings.Contains(text, "noop") == true {
			if disableNoOp == false {
				disableNoOp = true
			} else {
				disableNoOp = false
			}
		}
	}
}
