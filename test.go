package main

import (
	"fmt"
	"time"
)

func main() {
	ticker := time.NewTicker(1 * time.Second)
	for t := range ticker.C {
		fmt.Println(t)
	}
}
