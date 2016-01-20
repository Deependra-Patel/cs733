package main

import (
	"fmt"
	"sync"
	"net"
	"bufio"
	"strings"
	"strconv"
	"time"
	"log"
)

var lock = &sync.Mutex{}
var versionMap = make(map[string] int)
var contentMap = make(map[string] string)
var fileTimeMap = make(map[string] time.Time)
var expiryMap = make(map[string] time.Duration)
var infiTime = time.Second*1000000000
func write(filename string, numBytes int, seconds int, reader *bufio.Reader) string{
	buffer := ""
	total := 0
	for total < numBytes {
		curBytes, _, _ := reader.ReadLine()
		total += len(curBytes)
		buffer += string(curBytes)
	}

	lock.Lock()
	contentMap[filename] = buffer
	if versionMap[filename] == 0{
		versionMap[filename] = 1
	} else {
		versionMap[filename] += 1
	}
	fileTimeMap[filename] = time.Now()
	if(seconds == -1 || seconds == 0){
		expiryMap[filename] = infiTime
	} else {
		expiryMap[filename] = time.Second*time.Duration(seconds)
	}
	version := strconv.Itoa(versionMap[filename])
	lock.Unlock()
	return "OK "+string(version)+"\r\n"
}
func read(filename string) string{
	lock.Lock()
	defer lock.Unlock()
	if versionMap[filename] == 0{
		return "ERR_FILE_NOT_FOUND\r\n"
	}
	buffer := "CONTENTS " + strconv.Itoa(versionMap[filename]) + " " + strconv.Itoa(len(contentMap[filename])) + " "
	if expiryMap[filename]!=infiTime{
		if (fileTimeMap[filename].Add(expiryMap[filename])).Before(time.Now()){
			delete(versionMap, filename)
			delete(contentMap, filename)
			delete(expiryMap, filename)
			return "ERR_FILE_NOT_FOUND\r\n"
		}
		buffer += strconv.Itoa(int((fileTimeMap[filename].Add(expiryMap[filename])).Sub(time.Now()).Seconds()))
	}
	buffer += "\r\n" + contentMap[filename] + "\r\n"
	return buffer
}
func cas(filename string, version int, numBytes int, seconds int, reader *bufio.Reader) string{
	buffer := ""
	total := 0
	for total < numBytes {
		curBytes, _, _ := reader.ReadLine()
		total += len(curBytes)
		buffer += string(curBytes)
	}
	lock.Lock()
	defer lock.Unlock()
	if(versionMap[filename] == 0) {
		return "ERR_FILE_NOT_FOUND\r\n"
	} else if (fileTimeMap[filename].Add(expiryMap[filename])).Before(time.Now()){
		delete(versionMap, filename)
		delete(contentMap, filename)
		delete(expiryMap, filename)
		return "ERR_FILE_NOT_FOUND\r\n"
	} else if(versionMap[filename] != version){
		return "ERR_VERSION\r\n"
	} else {
		contentMap[filename] = buffer
		versionMap[filename] += 1
		fileTimeMap[filename] = time.Now()
		if(seconds == -1 || seconds == 0){
			expiryMap[filename] = infiTime
		} else {
			expiryMap[filename] = time.Second*time.Duration(seconds)
		}
		return "OK " + strconv.Itoa(version+1) + "\r\n"
	}
}

func deleteFile(filename string) string{
	buffer := ""
	lock.Lock()
	if versionMap[filename] == 0{
		buffer += "ERR_FILE_NOT_FOUND\r\n"
	} else {
		delete(versionMap, filename)
		delete(contentMap, filename)
		delete(expiryMap, filename)
		buffer += "OK\r\n"
	}
	lock.Unlock()
	return buffer
}

func requestHandler(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		firstLine, _, err := reader.ReadLine()
		fmt.Println(firstLine)
		firstLineTokens := strings.Split(string(firstLine), " ")

		if err != nil {
			return
		}
		fmt.Println(firstLineTokens, len(firstLineTokens))
		if firstLineTokens[0] == "write" {
			if len(firstLineTokens) != 3 && len(firstLineTokens) != 4 {
				conn.Close()
				return
			}
			if len(firstLineTokens) == 3 {
				numbytes, _ := strconv.Atoi(firstLineTokens[2])
				conn.Write([]byte(write(firstLineTokens[1], numbytes, -1, reader)))
			} else if len(firstLineTokens) == 4 {
				numbytes, _ := strconv.Atoi(firstLineTokens[2])
				exptime, _ := strconv.Atoi(firstLineTokens[3])
				conn.Write([]byte(write(firstLineTokens[1], numbytes, exptime, reader)))
			}
		} else if (firstLineTokens[0] == "read") {
			if (len(firstLineTokens) != 2) {
				conn.Close()
				return
			}
			conn.Write([]byte(read(firstLineTokens[1])))
		} else if firstLineTokens[0] == "cas" {
			if len(firstLineTokens) != 4 && len(firstLineTokens) != 5 {
				conn.Close()
				return
			}
			if len(firstLineTokens) == 4 {
				version, _ := strconv.Atoi(firstLineTokens[2])
				numbytes, _ := strconv.Atoi(firstLineTokens[3])
				conn.Write([]byte(cas(firstLineTokens[1], version, numbytes, -1, reader)))
			} else if len(firstLineTokens) == 5 {
				version, _ := strconv.Atoi(firstLineTokens[2])
				numbytes, _ := strconv.Atoi(firstLineTokens[3])
				exptime, _ := strconv.Atoi(firstLineTokens[4])
				conn.Write([]byte(cas(firstLineTokens[1], version, numbytes, exptime, reader)))
			}
		} else if firstLineTokens[0] == "delete" {
			if len(firstLineTokens) != 2 {
				conn.Close()
				return
			}
			filename := firstLineTokens[1]
			conn.Write([]byte(deleteFile(filename)))
		}
	}
}
func serverMain(){
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		// handle error
		fmt.Println("Server couldn't be started.")
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("More threads can't be created \n")
		}
		go requestHandler(conn)
	}
}
func main() {
	serverMain()
}
