package main

import (
	logger "log"
	"testing"
	"time"
	"strconv"
	"sync"
)

func TestBasic(t *testing.T) {
	rafts := makeRafts(t, 5, 7000) // array of []raftNode
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
		node.Delete()
	}
}

func TestNewLeader(t *testing.T) {
	rafts := makeRafts(t, 5, 7100) // array of []raftNode
	ldr := getLeader(t, rafts)

	first := "deep"
	second := "patel"

	ldr.Append([]byte("deep"))

	var wg sync.WaitGroup
	wg.Add(len(rafts))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != first {
				t.Fatal("Got different data")
			}
			err, data := node.Get(1)
			if (err != nil || string(data)!= first){
				t.Fatal("Expected message on log also")
			}
			logger.Println("Done ID:", node.Id())
		}(node)
	}
	wg.Wait()
	ldr.Shutdown()
	ldr.Delete()

	earlierLeader := ldr.Id()
	var wg2 sync.WaitGroup
	wg2.Add(len(rafts)-1)
	ldr = getLeader(t, rafts)
	ldr.Append([]byte(second))
	for _, node := range rafts{
		if (node.Id() == earlierLeader){
			continue
		}
		go func(node RaftNode){
			defer wg2.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != second {
				t.Fatal("Got different data")
			}
			err3, _ := node.Get(3)
			err2, data2 := node.Get(2)
			err1, data1 := node.Get(1)
			//fmt.Println("here", node.Id(), string(data1), err1, string(data2), err2, string(data3), err3)
			if (err1 != nil || string(data1)!= first || err2 != nil || string(data2)!= second || err3==nil){
				t.Fatal("Persistent log has unexpected values")
			}
		}(node)
	}
	wg2.Wait()

	for _, node := range rafts{
		if (node.Id() != earlierLeader) {
			node.Shutdown()
			node.Delete()
		}
	}
}

func TestRecovery(t *testing.T) {
	rafts := makeRafts(t, 5, 7200) // array of []raftNode
	ldr := getLeader(t, rafts)

	first := "deep"
	second := "patel"

	ldr.Append([]byte("deep"))

	var wg sync.WaitGroup
	wg.Add(len(rafts))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != first {
				t.Fatal("Got different data")
			}
			err, data := node.Get(1)
			if (err != nil || string(data)!= first){
				t.Fatal("Expected message on log also")
			}
			logger.Println("Done ID:", node.Id())
		}(node)
	}
	wg.Wait()

	for _, node := range rafts{
		node.Shutdown()
	}

	rafts = makeRafts(t, 5, 7200) // array of []raftNode

	var wg2 sync.WaitGroup
	wg2.Add(len(rafts))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg2.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != first {
				t.Fatal("Got different data")
			}
			err, data := node.Get(1)
			if (err != nil || string(data)!= first){
				t.Fatal("Expected message on log also")
			}
			logger.Println("Done ID:", node.Id())
		}(node)
	}
	wg2.Wait()

	var wg3 sync.WaitGroup
	wg3.Add(len(rafts))
	ldr = getLeader(t, rafts)
	ldr.Append([]byte(second))
	for _, node := range rafts{
		go func(node RaftNode){
			defer wg3.Done()
			ci := <- node.CommitChannel()
			if ci.err != "" {
				t.Fatal(ci.err)
			}
			if string(ci.data) != second {
				t.Fatal("Got different data")
			}
			err3, _ := node.Get(3)
			err2, data2 := node.Get(2)
			err1, data1 := node.Get(1)
			if (err1 != nil || string(data1)!= first || err2 != nil || string(data2)!= second || err3==nil){
				t.Fatal("Persistent log has unexpected values")
			}
		}(node)
	}
	wg3.Wait()

	for _, node := range rafts{
		node.Shutdown()
		node.Delete()
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

func getConfigs(n int, port int) []Config{
	netConfigs := make([]NetConfig, n)
	for i:=0; i<n; i++{
		netConfigs[i] = NetConfig{Id: i+1, Host: "localhost", Port: port+i}
	}

	config := Config{
		cluster : netConfigs,
		Id:               1,
		LogDir:           "mylog",
		ElectionTimeout:  time.Millisecond*time.Duration(1500),
		HeartbeatTimeout: time.Millisecond*time.Duration(500),
	}

	configs := make([]Config,0)
	for i := 0; i<n; i++ {
		temp := config
		temp.Id = i + 1
		temp.LogDir = temp.LogDir + strconv.Itoa(i + 1)
		configs = append(configs, temp)
	}
	return configs
}

func makeRafts(t *testing.T, n int, port int) []RaftNode{
	raftNodes := make([]RaftNode, n)

	configs := getConfigs(n, port)
	for i := 0; i<n; i++{
		raftNodes[i] = New(configs[i])
	}

	for _, raftNode := range(raftNodes){
		go func(raftNode RaftNode) {
			raftNode.processEvents()
		}(raftNode)
	}

	return raftNodes
}
