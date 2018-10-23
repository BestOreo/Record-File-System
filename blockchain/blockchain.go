package blockchain

import (
	"fmt"
	"sync"
)

// TX_BUFFER_SIZE is the maximum number of tx's from clients before a new block is created
var TX_BUFFER_SIZE = 3

// Tx represents a single transaction from a client
type Tx struct {
	opType  string
	content []byte
	minerID string
}

// Block is a single structure in the chain
type Block struct {
	prevHash     string
	nonce        string
	transactions []Tx
}

// BlockChain is the central datastructure
type BlockChain struct {
	chainLock    sync.Mutex
	chain        []Block
	txBuffer     []Tx
	txBufferSize int
	difficulty   int
}

func (bc *BlockChain) createBlock() {
	// assume that txBuffer is full
	for _, tx := range bc.txBuffer {
		fmt.Println(tx.opType)
	}
}

func (bc *BlockChain) createTransaction(opType string, content []byte, minerID string) {
	currBuff := len(bc.txBuffer)
	if currBuff < TX_BUFFER_SIZE {
		bc.chainLock.Lock()
		tx := Tx{
			opType,
			content,
			minerID,
		}
		bc.txBuffer[currBuff] = tx
		bc.chainLock.Unlock()
	} else {
		bc.createBlock()
	}
}

func (bc *BlockChain) printChain() {
	for _, block := range bc.chain {
		fmt.Println(block.prevHash)
	}
}

// For our example we'll implement this interface on
// `rect` and `circle` types.
// type rect struct {
// 	width, height float64
// }
// type circle struct {
// 	radius float64
// }

// // To implement an interface in Go, we just need to
// // implement all the methods in the interface. Here we
// // implement `geometry` on `rect`s.
// func (r rect) area() float64 {
// 	return r.width * r.height
// }
// func (r rect) perim() float64 {
// 	return 2*r.width + 2*r.height
// }

// // The implementation for `circle`s.
// func (c circle) area() float64 {
// 	return math.Pi * c.radius * c.radius
// }
// func (c circle) perim() float64 {
// 	return 2 * math.Pi * c.radius
// }

// // If a variable has an interface type, then we can call
// // methods that are in the named interface. Here's a
// // generic `measure` function taking advantage of this
// // to work on any `geometry`.
// func measure(g geometry) {
// 	fmt.Println(g)
// 	fmt.Println(g.area())
// 	fmt.Println(g.perim())
// }
