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
var blockQueue []*Block

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

/*******************************************/
// Queue Opearation
func pushRecordQueue(opmsg *OpMsg) {
	recordQueue = append(recordQueue, opmsg)
}

func popRecordQueue() *OpMsg {
	if len(recordQueue) == 0 {
		return nil
	}
	head := recordQueue[0]
	recordQueue = recordQueue[1:]
	return head
}

func pushBlockQueue(block *Block) {
	blockQueue = append(blockQueue, block)
}

func popBlockQueue() *Block {
	if len(blockQueue) == 0 {
		return nil
	}
	head := blockQueue[0]
	blockQueue = blockQueue[1:]
	return head
}

func printRecordQueue() {
	println("---------------------------\nRecordQueue")
	for i := 0; i < len(recordQueue); i++ {
		println(recordQueue[i].MinerID, recordQueue[i].MsgID, recordQueue[i].Op)
	}
	println("---------------------------")
}

func printBlockQueue() {
	println("---------------------------\nBlockQueue")
	for i := 0; i < len(blockQueue); i++ {
		println(blockQueue[i].PrevHash, blockQueue[i].Nonce)
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

func makeTimestamp() int {
	return int(time.Now().UnixNano() % 1e6 / 1e3)
}

// just a demo
func (t *MinerHandle) MinerTalk(args *ClientMsg, reply *int) error {
	*reply = 0
	println(args.Operation, "from", args.MinerID)
	return nil
}

// In order to avoid message flooding in loop network
func checkOperationInQueue(msg *OpMsg) bool {
	for i := 0; i < len(recordQueue); i++ {
		if recordQueue[i].MinerID == msg.MinerID && recordQueue[i].MsgID == msg.MsgID {
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

/******************************************/
// RPC Handler

// FloodOperation : flood operation of client to the whole network
func (t *MinerHandle) FloodOperation(record *OpMsg, reply *int) error {
	*reply = 0
	// println("------------")
	// println("| Msg: ", record.MinerID, record.MsgID, record.Op, record.Name, record.Content)
	// println("------------")
	if checkOperationInQueue(record) == false {
		printColorFont("green", "pushed into recordQueue")
		pushRecordQueue(record)
		minerChain.createTransaction(record.Op, record.Name, record.Content, record.MinerID)
		broadcastOperations(*record)
	}
	return nil
}

// FloodOperation : flood block to the whole network
func (t *MinerHandle) FloodBlock(block *Block, reply *int) error {
	*reply = 0
	println("------------")
	println("| HASHMsg: ")
	fmt.Printf("%v\n", block)
	for _, tx := range block.Transactions {
		fmt.Printf("%v\n", tx)
	}
	println("------------")
	if checkBlockInQueue(block) == false {
		printColorFont("green", "pushed into blockQueue")
		pushBlockQueue(block)
		minerChain.addBlockToChain(block)
		broadcastBlocks(block)
	}
	return nil
}

/******************************************/

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
			fmt.Printf("%%% send %v to %s successfully\n", operationMsg, ip)
		}
	}
}

// broadcastBlocks broadcast blcok to whole network
func broadcastBlocks(block *Block) {
	fmt.Println("BROADCAST ----")
	fmt.Printf("%v\n", block)
	for _, tx := range block.Transactions {
		fmt.Printf("%v\n", tx)
	}
	fmt.Println("END BROADCAST----")
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
			fmt.Printf("--- send %v to %s successfully\n", block, ip)
		}
	}
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
						minerChain.createTransaction(msgjson["op"], msgjson["name"], msgjson["Content"], config.MinerID)
						// codes about blockchain
						conn.Write([]byte("success"))
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["Content"])
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
					} else if len(msgjson["Content"])/512 > 655354 { // have at most 65,5354 (uint16) records
						conn.Write([]byte("FileMaxLenReachedError"))
					} else {
						var m [512]byte
						copy(m[:], []byte(msgjson["Content"]))
						blockFile[msgjson["name"]] += string(m[:])

						// codes about blockchain

						conn.Write([]byte(strconv.Itoa(len(blockFile[msgjson["name"]])/512 - 1)))
						operationMsg := generateOpMsg(msgjson["op"], msgjson["name"], msgjson["Content"])
						pushRecordQueue(&operationMsg)
						broadcastOperations(operationMsg)
					}
				}
			}
		}()
	}
}

/*** Blockchain ***/

// TX_BUFFER_SIZE is the maximum number of tx's from clients before a new block is created
var TX_BUFFER_SIZE = 2

// Tx represents a single transaction from a client
type Tx struct {
	OpType   string
	filename string
	content  string
	MinerID  string
}

// Block is a single structure in the chain
type Block struct {
	PrevHash     string
	Index        int
	Timestamp    int
	Nonce        uint32
	Transactions []Tx
}

// BlockChain is the central datastructure
type BlockChain struct {
	chainLock    *sync.Mutex
	chain        []*Block
	txBuffer     []Tx
	txBufferSize int
	difficulty   int
}

func (bc *BlockChain) init() {
	// create genesis block
	block := &Block{}
	block.PrevHash = ""
	block.Nonce = 0
	bc.chain = append(bc.chain, block)
	fmt.Println("Genisis block created.")
	go minerChain.printChain()
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
		fmt.Println("incorrect prev hash")
		fmt.Println(block.PrevHash)
		fmt.Println(lastBlockHash)
	}
	// hashing should produce correct number of zeros
	hasCorrectHash := strings.HasSuffix(blockHash, numberOfZeros)
	if !hasCorrectHash {
		fmt.Println("---")
		fmt.Println(blockHash)
		fmt.Println(block.Nonce)
		fmt.Println("incorrect hash")
	}
	// validate all the transactions
	hasCorrectTxns := true
	isValidBlock = hasCorrectHash && hasCorrectTxns && hasCorrectPrevHash
	return isValidBlock
}
func (bc *BlockChain) addBlockToChain(block *Block) {
	// add it to the chain
	bc.chainLock.Lock()
	if bc.verifyBlock(block) {
		bc.chain = append(bc.chain, block)
	} else {
		fmt.Println("Block not verified.")
	}
	bc.chainLock.Unlock()

	fmt.Println("Block Added")
}

func (bc *BlockChain) createBlock() {
	// set prev hash
	block := &Block{}
	block.PrevHash = bc.hashBlock(bc.chain[len(bc.chain)-1])
	block.Transactions = bc.txBuffer
	block.Timestamp = makeTimestamp()
	block.Index = len(bc.chain)
	println("%%%%%%%%%%%%%%5")
	for _, v := range block.Transactions {
		fmt.Printf("%v\n", v)
	}
	println("%%%%%%%%%%%%%%5")
	if len(bc.txBuffer) > TX_BUFFER_SIZE {
		bc.txBuffer = bc.txBuffer[TX_BUFFER_SIZE:]
	} else {
		bc.txBuffer = make([]Tx, 0)
	}
	// mine the block to find solution
	block.Nonce = bc.proofOfWork(block)
	bc.addBlockToChain(block)
	broadcastBlocks(block)
}

func (bc *BlockChain) proofOfWork(block *Block) (Nonce uint32) {
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
	fmt.Println(block.PrevHash)
	fmt.Println(str)
	fmt.Println(Nonce)
	return Nonce
}

func (bc *BlockChain) hashBlock(block *Block) (str string) {
	hash := md5.New()
	blockData := bc.getBlockBytes(block)
	hash.Write(blockData)
	str = hex.EncodeToString(hash.Sum(nil))
	return str
}

func (bc *BlockChain) createTransaction(OpType string, fileName string, content string, MinerID string) {
	bc.chainLock.Lock()
	tx := Tx{OpType, fileName, content, MinerID}
	bc.txBuffer = append(bc.txBuffer, tx)
	bc.chainLock.Unlock()
	if len(bc.txBuffer) == TX_BUFFER_SIZE {
		bc.createBlock()
	}
}

func (bc *BlockChain) getBlockBytes(block *Block) []byte {
	txString := ""

	for _, tx := range block.Transactions {
		txString = tx.OpType + tx.MinerID
	}

	// timeBytes, err := time.Now().MarshalJSON()
	// if err != nil {
	// 	panic("time failed")
	// }
	data := bytes.Join(
		[][]byte{
			[]byte(block.PrevHash),
			[]byte(fmt.Sprint(strconv.Itoa(block.Index))),
			[]byte(fmt.Sprint(strconv.Itoa(block.Timestamp))),
			[]byte(txString),
			[]byte(fmt.Sprint(block.Nonce)),
		},
		[]byte{},
	)
	return data
}

// maps blockprevhash -> index of content in the block that have the filename
func (bc *BlockChain) findFile(fileName string) (blockTxMap map[string][]int) {
	blockTxMap = make(map[string][]int)
	for _, block := range bc.chain {
		for txID, tx := range block.Transactions {
			hasBlock := false
			if tx.filename == fileName {
				if !hasBlock {
					blockTxMap[block.PrevHash] = make([]int, 0)
					hasBlock = true
				}
				blockTxMap[block.PrevHash] = append(blockTxMap[block.PrevHash], txID)
				fmt.Println(block.PrevHash)
				fmt.Println(tx.filename)
			}
		}
	}
	fmt.Println("map:", blockTxMap)
	return blockTxMap
}
func (bc *BlockChain) findFiles(fileName string) {
	blockTxMap := make(map[string][]int)
	for _, block := range bc.chain {
		for txID, tx := range block.Transactions {
			hasBlock := false
			if tx.filename == fileName {
				if !hasBlock {
					blockTxMap[block.PrevHash] = make([]int, 0)
					hasBlock = true
				}
				blockTxMap[block.PrevHash] = append(blockTxMap[block.PrevHash], txID)
				fmt.Println(block.PrevHash)
				fmt.Println(tx.filename)
			}
		}
	}
	fmt.Println("map:", blockTxMap)
}
func (bc *BlockChain) printChain() {
	idx := 0
	for now := range time.Tick(10 * time.Second) {
		idx++
		if idx == 2 {
			bc.findFile("text4.txt")
		}
		fmt.Println("CURRENT CHAIN : ---------------------------")
		fmt.Println(now)
		for idx, block := range bc.chain {
			fmt.Println("****************************")
			fmt.Println("Block Number: ", idx)
			fmt.Println("Prev Hash: ", block.PrevHash)
			fmt.Println("Transactions: ")
			for _, tx := range block.Transactions {
				fmt.Printf("%v\n", tx)
			}
			fmt.Println("****************************")
		}
	}
}

func Initial() {
	blockFile = make(map[string]string)
	recordQueue = make([]*OpMsg, 0)
	blockQueue = make([]*Block, 0)
	minerChain = &BlockChain{
		chainLock:    &sync.Mutex{},
		chain:        make([]*Block, 0),
		txBuffer:     make([]Tx, 0),
		txBufferSize: 10,
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
		} else if strings.Contains(text, "pop") == true {
			rec := popRecordQueue()
			if rec == nil {
				println("The queue is empty")
			}
		} else if strings.Contains(text, "floodblock") == true {
			broadcastBlocks(&Block{"Hello", 0, 0, 65535, nil})
		}
	}
}
