package main

import (
	logger "log"
	"testing"
	"time"
	"strconv"
	"sync"
	"fmt"
)

func TestBasic(t *testing.T) {
	logger.SetFlags(logger.LstdFlags | logger.Lshortfile)
	rafts := makeRafts(t, 5) // array of []raftNode
	ldr := getLeader(t, rafts)
	ldr.Append([]byte("foo"))

	var wg sync.WaitGroup
	wg.Add(len(rafts))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
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
	for _, node := range rafts{
		node.Shutdown()
	}
}

func TestNewLeader(t *testing.T) {
	logger.SetFlags(logger.LstdFlags | logger.Lshortfile)
	rafts := makeRafts(t, 5) // array of []raftNode
	ldr := getLeader(t, rafts)
	ldr.Append([]byte("foo"))

	var wg sync.WaitGroup
	wg.Add(len(rafts))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != "foo" {
				t.Fatal("Got different data")
			}
			err, data := node.Get(1)
			if (err != nil || string(data)!= "foo"){
				t.Fatal("Expected message on log also")
			}
			logger.Println("Done ID:", node.Id())
		}(node)
	}
	wg.Wait()

	ldr.Shutdown()

	earlierLeader := ldr.Id()
	var wg2 sync.WaitGroup
	wg2.Add(len(rafts)-1)
	ldr = getLeader(t, rafts)
	ldr.Append([]byte("bar"))
	for _, node := range rafts{
		if (node.Id() == earlierLeader){
			continue
		}
		go func(node RaftNode){
			defer wg2.Done()
			ci := <- node.CommitChannel()
			fmt.Println("Finished", node.Id())
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != "bar" {
				t.Fatal("Got different data")
			}
			err3, _ := node.Get(3)
			err2, data2 := node.Get(2)
			err1, data1 := node.Get(1)
			//fmt.Println("here", node.Id(), string(data1), err1, string(data2), err2, string(data3), err3)
			if (err1 != nil || string(data1)!= "foo" || err2 != nil || string(data2)!= "bar" || err3==nil){
				t.Fatal("Persistent log has unexpected values")
			}
		}(node)
	}
	wg2.Wait()

	for _, node := range rafts{
		if (node.Id() != earlierLeader) {
			node.Shutdown()
		}
	}
}

func getLeader(t *testing.T, rafts []RaftNode) RaftNode{
	for {
		for _, raft := range (rafts) {
			if (raft.sm.state == "Leader" && !raft.server.IsClosed()) {
				return raft
			}
		}
		time.Sleep(100*time.Millisecond)
	}
}

func makeRafts(t *testing.T, n int) []RaftNode{
	raftNodes := make([]RaftNode, n)
	netConfigs := make([]NetConfig, n)
	for i:=0; i<n; i++{
		netConfigs[i] = NetConfig{Id: i+1, Host: "localhost", Port: 7000+i+1}
	}

	config := Config{
		cluster : netConfigs,
		Id:               1,
		LogDir:           "mylog",
		ElectionTimeout:  time.Millisecond*time.Duration(1500),
		HeartbeatTimeout: time.Millisecond*time.Duration(500),
	}

	for i := 0; i<n; i++{
		temp := config
		temp.Id = i+1
		temp.LogDir = temp.LogDir + strconv.Itoa(i+1)
		raftNodes[i] = New(temp)
	}

	for _, raftNode := range(raftNodes){
		go func(raftNode RaftNode) {
			raftNode.processEvents()
		}(raftNode)
	}

	return raftNodes
}
