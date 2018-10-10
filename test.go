package main

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"log"
)

func GetAllFile(pathname string) error {
	filelist := list.New()
	dir, err := ioutil.ReadDir(pathname)
	for _, file := range dir {
		if file.IsDir() {
			GetAllFile(pathname + "/" + file.Name() + "/")
		} else {
			filelist.PushBack(file.Name())
		}
	}

	return err
}

func ListFiles() {
	path := "127.0.0.1:8000-127.0.0.1:9000"
	dir, _ := ioutil.ReadDir(path)
	for _, file := range dir {
		println(file.Name())
	}
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

func main() {
	s := readFileByte("a.txt")
	fmt.Print(len(s))
}
