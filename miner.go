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
			println("timestamp:", block.Timestamp)
			println("------------")
			println()
			pushBlockQueue(block)
			minerChain.addBlockToChain(block)
			broadcastBlocks(block)
		}
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
				fmt.Println(msgjson["op"], msgjson["name"], msgjson["Content"])

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
						conn.Write([]byte("success"))
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["content"])
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
						conn.Write([]byte(strconv.Itoa(len(blockFile[msgjson["name"]]) / 512)))
					}
					// Client ReadRec
				} else if msgjson["op"] == "ReadRec" {
					pos, err := strconv.Atoi(msgjson["Content"])
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
						pushRecordQueue(&operationMsg)
						broadcastOperations(operationMsg)
					}
				}
			}
		}()
	}
}

/*** Blockchain ***/

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
	block.PrevHash = ""
	block.Nonce = 0
	bc.chain = append(bc.chain, block)
	fmt.Println("Genisis block created.")

	bc.startBlockGeneration()

	//bc.manageChain()
}

func (bc *BlockChain) startBlockGeneration() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for t := range ticker.C {
			fmt.Println(t)
			if len(recordQueue) == 0 {
				bc.createNoOpBlock()
			} else {
				bc.createTransactionBlock()
			}
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
	lastBlock := bc.chain[len(bc.chain)-1]
	lastBlockHash := bc.hashBlock(lastBlock)
	// should point to last block hash
	hasCorrectPrevHash := block.PrevHash == lastBlockHash
	if !hasCorrectPrevHash {
		fmt.Println("Block.PrevHash:", block.PrevHash)
		fmt.Println("Real PrevHash", lastBlockHash)
		fmt.Println("Hint: Incorrect prev hash")
	}
	// hashing should produce correct number of zeros
	hasCorrectHash := strings.HasSuffix(blockHash, numberOfZeros)
	if !hasCorrectHash {
		fmt.Println("BlockHash:\t", blockHash)
		fmt.Println("BlockNonce:\t", block.Nonce)
		fmt.Println("Hint: Incorrect hash")
	}
	// validate all the transactions
	transactions := convertJsonArray(block.Transactions)
	hasvalidTransactions := true
	for i := 0; i < len(transactions); i++ {
		json := transactions[i]
		if !bc.validateTransactions(json["filename"], json["op"], json["content"]) {
			hasvalidTransactions = false
		}
		if !bc.validateCoinBalance(json["minerId"]) {
			hasvalidTransactions = false
		}
	}
	// validate coin balance

	isValidBlock = hasCorrectHash && hasCorrectPrevHash && hasvalidTransactions
	return isValidBlock
}
func (bc *BlockChain) validateCoinBalance(minerID string) (valid bool) {
	return true
}
func (bc *BlockChain) addBlockToChain(block *Block) {
	// add it to the chain
	bc.chainLock.Lock()
	println(getTime())
	println("----------verifyBlock----------")
	if bc.verifyBlock(block) {
		bc.chain = append(bc.chain, block)
		fmt.Println("Result: Block Added")
	} else {
		fmt.Println("Result: Block not verified.")
	}
	println("----------End verifyBlock----------")
	println()
	bc.chainLock.Unlock()

}

// when timeout, you just use API createBlock
// the function will read records from the queue
// and generate a new block
func (bc *BlockChain) createTransactionBlock() {
	// set prev hash
	block := &Block{}
	block.PrevHash = bc.hashBlock(bc.chain[len(bc.chain)-1])
	// block.Transactions = bc.txBuffer
	block.Timestamp = makeTimestamp()
	block.Index = len(bc.chain)
	block.Miner = config.MinerID

	var transactionNum int
	// there are two cases
	if len(recordQueue) > bc.maxRecordNum {
		transactionNum = bc.maxRecordNum
	} else {
		transactionNum = len(recordQueue)
	}

	for i := 0; i < transactionNum; i++ {
		record := popRecordQueue()
		if len(block.Transactions) == 0 {
			block.Transactions = record.Op + "{,}" + record.Name + "{,}" + record.Content + "{,}" + record.MinerID
		} else {
			block.Transactions += "{;}" + record.Op + "{,}" + record.Name + "{,}" + record.Content + "{,}" + record.MinerID
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

	bc.addBlockToChain(block)
	broadcastBlocks(block)
}

func (bc *BlockChain) createNoOpBlock() {
	// set prev hash
	block := &Block{}
	block.PrevHash = bc.hashBlock(bc.chain[len(bc.chain)-1])
	// block.Transactions = bc.txBuffer
	block.Timestamp = makeTimestamp()
	block.Index = len(bc.chain)
	block.Transactions = "No-Op" + "{,}" + "{,}" + "{,}" + config.MinerID
	block.Miner = config.MinerID

	// mine the block to find solution
	block.Nonce = bc.proofOfWork(block)
	fmt.Printf("Block is %v\n", block)

	bc.addBlockToChain(block)
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
		} else {
			json["filename"] = elements[1]
			json["content"] = elements[2]
			json["minerId"] = elements[3]
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
		}
	}
}
