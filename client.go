package main

import (
	"./rfslib"
)

func main() {
	rfs, _ := rfslib.Initialize("127.0.0.1:8000", "127.0.0.1:9000")
	// err := rfs.CreateFile("test.txt")
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
	s := [512]byte{'1', '2'}
	rfs.AppendRec("text", s)
}
