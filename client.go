package main

import (
	"log"

	"./rfslib"
)

func main() {
	rfs, _ := rfslib.Initialize("127.0.0.1:8000", "127.0.0.1:9090")

	// err := rfs.CreateFile("text3.txt")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	ListFiles, err := rfs.ListFiles()
	if err != nil {
		log.Fatal(err)
	}
	for _, i := range ListFiles {
		println(i)
	}

	// s := rfslib.Record{'a', 'b', 'c', 'd'}
	// l, err := rfs.AppendRec("text.txt", &s)
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	println(l)
	// }

	// l, err := rfs.TotalRecs("text.txt")
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	println(l)
	// }

	// var m rfslib.Record
	// err := rfs.ReadRec("text.txt", 0, &m)
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	fmt.Print(m)
	// }

}
