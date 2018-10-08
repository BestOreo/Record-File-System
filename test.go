package main

import (
	"container/list"
	"fmt"
	"io/ioutil"
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

func main() {
	s := [512]byte{1, 2}
	fmt.Print(s)
}
