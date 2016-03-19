package main

import (
	logger "log"
	"testing"
	"time"
	"strconv"
	"fmt"
	"sync"
)

func TestBasic(t *testing.T) {
	logger.SetFlags(logger.LstdFlags | logger.Lshortfile)
	rafts := makeRafts(t) // array of []raftNode
	time.Sleep(time.Duration(1)*time.Second)
	ldr := getLeader(t, rafts)
	ldr.Append([]byte("foo"))
	time.Sleep(time.Duration(3)*time.Second)

	var wg sync.WaitGroup
	wg.Add(len(rafts))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			fmt.Println(string(ci.data))
			if string(ci.data) != "foo" {
				t.Fatal("Got different data")
			}
			err, data := node.Get(1)
			if (err != nil || string(data)!= "foo"){
				t.Fatal("Expected message on log also")
			}
		}(node)
	}
	wg.Wait()
}

func getLeader(t *testing.T, rafts []RaftNode) RaftNode{
	for {
		for _, raft := range (rafts) {
			if (raft.sm.state == "Leader") {
				return raft
			}
		}
		time.Sleep(100*time.Millisecond)
	}
}

func makeRafts(t *testing.T) []RaftNode{
	raftNodes := make([]RaftNode, 5)
	config := Config{
		cluster: []NetConfig{
			{Id: 1, Host: "localhost", Port: 7001},
			{Id: 2, Host: "localhost", Port: 7002},
			{Id: 3, Host: "localhost", Port: 7003},
			{Id: 4, Host: "localhost", Port: 7004},
			{Id: 5, Host: "localhost", Port: 7005},
		},
		Id:               1,
		LogDir:           "mylog",
		ElectionTimeout:  time.Millisecond*time.Duration(3000),
		HeartbeatTimeout: time.Millisecond*time.Duration(1000),
	}

	for i := 0; i<5; i++{
		temp := config
		temp.Id = i+1
		temp.LogDir = temp.LogDir + strconv.Itoa(i+1)
		fmt.Println(temp)
		raftNodes[i] = New(temp)
	}

	for _, raftNode := range(raftNodes){
		go func(raftNode RaftNode) {
			raftNode.processEvents()
		}(raftNode)
	}

	return raftNodes
}
