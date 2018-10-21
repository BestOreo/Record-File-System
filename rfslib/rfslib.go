/*

This package specifies the application's interface to the distributed
records system (RFS) to be used in project 1 of UBC CS 416 2018W1.

You are not allowed to change this API, but you do have to implement
it.

*/

package rfslib

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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

func json(op string, name string, content string) string {
	if content == "nil" {
		content = "null"
	}
	if name == "nil" {
		name = "null"
	}
	res := "{\"op\":\"" + op + "\",\"name\":\"" + name + "\",\"content\":\"" + content + "\"}"
	// println(res)
	return res
}

func sendTCP(remoteIPPort string, content string) (string, error) {
	conn, err := net.Dial("tcp", remoteIPPort)
	defer conn.Close() /// wait
	if err != nil {
		return "", DisconnectedError(remoteIPPort)
	}

	conn.Write([]byte(content))
	buf := make([]byte, 1024)
	c, err := conn.Read(buf)
	if err != nil {
		return "", DisconnectedError(remoteIPPort)
	}
	// fmt.Println("Reply:", string(buf[0:c]))

	return string(buf[0:c]), nil
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
	if len(fname) > 64 {
		return BadFilenameError(fname)
	}
	reply, err := sendTCP(f.minerAddr, json("CreateFile", fname, "nil"))
	if reply == "FileExistsError" {
		return FileExistsError(fname)
	}
	return err
}

func (f RFSInstance) ListFiles() ([]string, error) {
	reply, err := sendTCP(f.minerAddr, json("ListFiles", "nil", "nil"))
	if err != nil {
		return nil, err
	}
	if len(reply) == 0 {
		return nil, nil
	}
	list := strings.Split(reply, ";")
	return list, err
}

func (f RFSInstance) TotalRecs(fname string) (numRecs uint16, err error) {
	reply, err := sendTCP(f.minerAddr, json("TotalRecs", fname, "nil"))
	if err != nil {
		return 0, err
	}
	if reply == "FileDoesNotExistError" {
		return 0, FileDoesNotExistError(fname)
	}
	l, _ := strconv.Atoi(reply)
	return uint16(l), err
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
	reply, err := sendTCP(f.minerAddr, json("ReadRec", fname, strconv.Itoa(int(recordNum))))
	if err != nil {
		return err
	} else if reply == "FileDoesNotExistError" {
		return FileDoesNotExistError(fname)
	} else if reply == "RecordDoesNotExistError" {
		return RecordDoesNotExistError(recordNum)
	} else {
		copy((*record)[:], reply)
	}
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
	m := [512]byte(*record)
	var i int
	for i = 0; i < 512; i++ {
		if m[i] == 0 { // json can't have \x00, so we need to find the end of the record and discard all \x00
			break // e.g. the record is ['a','b',0,0,0,0,0,...,0,0,0],we only send "ab" as string
		}
	}
	content := string(m[0:i])
	reply, err := sendTCP(f.minerAddr, json("AppendRec", fname, string(content)))
	if err != nil {
		return 0, err
	}
	if reply == "FileDoesNotExistError" {
		return 0, FileDoesNotExistError(fname)
	} else if reply == "FileMaxLenReachedError" {
		return 0, FileMaxLenReachedError(fname)
	}
	l, _ := strconv.Atoi(reply)
	return uint16(l), err
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
	conn, err := net.Dial("tcp", minerAddr)
	if err != nil {
		return nil, DisconnectedError(minerAddr)
	}
	defer conn.Close() /// wait

	return RFSInstance{localAddr, minerAddr}, nil
}
