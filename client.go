package main

import (
	"fmt"
	"log"

	"./rfslib"
)

func main() {
	rfs, _ := rfslib.Initialize("127.0.0.1:8000", "127.0.0.1:9090")

	// err := rfs.CreateFile("text.txt")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// ListFiles, err := rfs.ListFiles()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// for _, i := range ListFiles {
	// 	println(i)
	// }

	// s := rfslib.Record{'a', 'b', 'c', 'd', 'e', 'f'}
	// l, err := rfs.AppendRec("text.txt", &s)
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	println("position", l)
	// }

	// l, err := rfs.TotalRecs("text2.txt")
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	println("Total records:", l)
	// }

	var m rfslib.Record
	err := rfs.ReadRec("text2.txt", 4, &m)
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("[512]byte:\n", m)
		fmt.Printf("string:\n%s\n", m)
	}

}
