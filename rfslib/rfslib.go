/*

This package specifies the application's interface to the distributed
records system (RFS) to be used in project 1 of UBC CS 416 2018W1.

You are not allowed to change this API, but you do have to implement
it.

*/

package rfslib

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
)

// A Record is the unit of file access (reading/appending) in RFS.
type Record [512]byte

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// Contains minerAddr
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("RFS: Disconnected from the miner [%s]", string(e))
}

// Contains recordNum that does not exist
type RecordDoesNotExistError uint16

func (e RecordDoesNotExistError) Error() string {
	return fmt.Sprintf("RFS: Record with recordNum [%d] does not exist", e)
}

// Contains filename. The *only* constraint on filenames in RFS is
// that must be at most 64 bytes long.
type BadFilenameError string

func (e BadFilenameError) Error() string {
	return fmt.Sprintf("RFS: Filename [%s] has the wrong length", string(e))
}

// Contains filename
type FileDoesNotExistError string

func (e FileDoesNotExistError) Error() string {
	return fmt.Sprintf("RFS: Cannot open file [%s] in D mode as it does not exist locally", string(e))
}

// Contains filename
type FileExistsError string

func (e FileExistsError) Error() string {
	return fmt.Sprintf("RFS: Cannot create file with filename [%s] as it already exists", string(e))
}

// Contains filename
type FileMaxLenReachedError string

func (e FileMaxLenReachedError) Error() string {
	return fmt.Sprintf("RFS: File [%s] has reached its maximum length", string(e))
}

/*
Name: PathExists
@ para: path string
@ Return: bool
Func: return ture if there exists a file according to path or return false if not
*/
func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

/*
Name: makedir
@ para: path string
@ Return: None
Func: build a new dictionary if there is no correspondinng dictionary
*/
func makedir(path string) {
	if PathExists(path) == true {
		return
	}
	err := os.Mkdir(path, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

/*
Name: writeFile
@ para: filePath string
@ para: content string
@ para: appendEnable string
@ Return: None
Func: write the string content into assigned path by method of overwriting or appending
*/
func writeFile(filePath string, content []byte, appendEnable bool) error {
	if appendEnable == false {
		f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			log.Fatal(err)
			return err
		}
		f.Write(content)
	} else {
		f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
		if err != nil {
			log.Fatal(err)
			return err
		}
		f.Write(content)
	}
	return nil
}

/*
Name: readFileByte
@ para: filePath string
@ Return: string
Func: read and then return the byte of content from file in corresponding path
*/
func readFileByte(filePath string) ([]byte, error) {
	if PathExists(filePath) == false {
		return nil, FileDoesNotExistError(filePath)
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	return data, nil
}

func sendTCP(remoteIPPort string, content string) error {
	conn, err := net.Dial("tcp", remoteIPPort)
	if err != nil {
		return DisconnectedError(remoteIPPort)
	}
	// fmt.Println("已连接服务器")

	conn.Write([]byte(content))
	buf := make([]byte, 128)
	c, err := conn.Read(buf)
	if err != nil {
		return DisconnectedError(remoteIPPort)
	}
	fmt.Println(string(buf[0:c]))
	return nil
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

// Represents a connection to the RFS system.
type RFS interface {
	// Creates a new empty RFS file with name fname.
	//
	// Can return the following errors:
	// - DisconnectedError
	// - FileExistsError
	// - BadFilenameError
	CreateFile(fname string) (err error)

	// Returns a slice of strings containing filenames of all the
	// existing files in RFS.
	//
	// Can return the following errors:
	// - DisconnectedError
	ListFiles() (fnames []string, err error)

	// Returns the total number of records in a file with filename
	// fname.
	//
	// Can return the following errors:
	// - DisconnectedError
	// - FileDoesNotExistError
	TotalRecs(fname string) (numRecs uint16, err error)

	// Reads a record from file fname at position recordNum into
	// memory pointed to by record. Returns a non-nil error if the
	// read was unsuccessful.
	//
	// Can return the following errors:
	// - DisconnectedError
	// - FileDoesNotExistError
	// - RecordDoesNotExistError (indicates record at this position has not been appended yet)
	ReadRec(fname string, recordNum uint16, record *Record) (err error)

	// Appends a new record to a file with name fname with the
	// contents pointed to by record. Returns the position of the
	// record that was just appended as recordNum. Returns a non-nil
	// error if the operation was unsuccessful.
	//
	// Can return the following errors:
	// - DisconnectedError
	// - FileDoesNotExistError
	// - FileMaxLenReachedError
	AppendRec(fname string, record *Record) (recordNum uint16, err error)
}

type RFSInstance struct {
	localAddr string
	minerAddr string
}

// Can return the following errors:
// - DisconnectedError
// - FileExistsError
// - BadFilenameError
func (f RFSInstance) CreateFile(fname string) (err error) {
	filePath := "./" + f.localAddr + "-" + f.minerAddr + "/" + fname
	if len(fname) > 64 {
		return BadFilenameError(fname)
	} else if PathExists(filePath) == true {
		return FileExistsError(fname)
	}
	sendTCP(f.minerAddr, "CreateFile "+fname)
	return writeFile(filePath, []byte(""), false)
}

func (f RFSInstance) ListFiles() (fnames []string, err error) {
	path := f.localAddr + "-" + f.minerAddr
	dir, _ := ioutil.ReadDir(path)
	count := 0
	fnames = make([]string, len(dir))
	for _, file := range dir {
		fnames[count] = file.Name()
		count++
	}
	err = sendTCP(f.minerAddr, "ListFiles()")
	return fnames, err
}

func (f RFSInstance) TotalRecs(fname string) (numRecs uint16, err error) {
	filePath := "./" + f.localAddr + "-" + f.minerAddr + "/" + fname
	data, err := readFileByte(filePath)
	if err != nil {
		return 0, err
	}
	length := uint16(len(data)/512 - 1)
	err = sendTCP(f.minerAddr, "TotalRecs "+fname)
	return length, err
}

// Reads a record from file fname at position recordNum into
// memory pointed to by record. Returns a non-nil error if the
// read was unsuccessful.
//
// Can return the following errors:
// - DisconnectedError
// - FileDoesNotExistError
// - RecordDoesNotExistError (indicates record at this position has not been appended yet)
func (f RFSInstance) ReadRec(fname string, recordNum uint16, record *Record) (err error) {
	filePath := "./" + f.localAddr + "-" + f.minerAddr + "/" + fname
	data, err := readFileByte(filePath)
	if err != nil {
		return err
	}
	if len(data) < (int(recordNum)+1)*512 {
		return RecordDoesNotExistError(recordNum)
	}
	var m [512]byte
	copy(m[:], data[recordNum*512:recordNum*512+512])
	*record = Record(m)

	err = sendTCP(f.minerAddr, "ReadRec "+fname+" "+strconv.Itoa(int(recordNum)))
	return err
}

// Appends a new record to a file with name fname with the
// contents pointed to by record. Returns the position of the
// record that was just appended as recordNum. Returns a non-nil
// error if the operation was unsuccessful.
//
// Can return the following errors:
// - DisconnectedError
// - FileDoesNotExistError
// - FileMaxLenReachedError
func (f RFSInstance) AppendRec(fname string, record *Record) (recordNum uint16, err error) {
	path := f.localAddr + "-" + f.minerAddr + "/" + fname
	if PathExists(path) == false {
		return 0, FileDoesNotExistError(fname)
	}
	m := [512]byte(*record)
	content := []byte(m[:])
	oridata, err := readFileByte(path)
	if err != nil {
		return 0, err
	}
	err = writeFile(path, content, true)
	if err != nil {
		return 0, err
	}
	data, err := readFileByte(path)
	if err != nil {
		return 0, err
	}
	length := uint16(len(data)/512 - 1)
	err = sendTCP(f.minerAddr, "AppendRec "+fname)
	if err != nil {
		// if unable to connect miner, then roll back
		writeFile(path, oridata, false)
		return 0, err
	}
	return length, err
}

// The constructor for a new RFS object instance. Takes the miner's
// IP:port address string as parameter, and the localAddr which is the
// local IP:port to use to establish the connection to the miner.
//
// The returned rfs instance is singleton: an application is expected
// to interact with just one rfs at a time.
//
// This call should only succeed if the connection to the miner
// succeeds. This call can return the following errors:
// - Networking errors related to localAddr or minerAddr
func Initialize(localAddr string, minerAddr string) (rfs RFS, err error) {
	// TODO
	// For now return a DisconnectedError
	makedir(localAddr + "-" + minerAddr)
	return RFSInstance{localAddr, minerAddr}, nil
}
