package main

import (
	logger "log"
	"testing"
)

func TestBasic(t *testing.T) {
	logger.SetFlags(logger.LstdFlags | logger.Lshortfile)
	r1 := New(
		Config{
			cluster: []NetConfig{
				{Id: 1, Host: "localhost", Port: 8000},
				{Id: 2, Host: "localhost", Port: 8001},
				{Id: 3, Host: "localhost", Port: 8002},
			},
			Id:               1,
			LogDir:           "mylog",
			ElectionTimeout:  2,
			HeartbeatTimeout: 1,
		})
	r1.sm.state = "Leader"
	r1.Append([]byte("hi deependra"))
	//commitChan := r1.CommitChannel()

	//rafts := makeRafts() // array of []raft.Node
	//ldr := getLeader(rafts)
	//ldr.Append("foo")
	//time.Sleep(1*time.Second)
	//for _, node:= range rafts {
	//select {
	//// to avoid blocking on channel.
	//case ci := <- node.CommitChannel():
	//	if ci.err != nil {t.Fatal(ci.err)}
	//	if string(ci.data) != "foo" {
	//		t.Fatal("Got different data")
	//	}
	//default: t.Fatal("Expected message on all nodes")
	//	}
	//}
}

//func getLeader(rafts []RaftNode) RaftNode{
//	for _, raft := range(rafts){
//		if (raft.sm.state == "Leader"){
//			return raft
//		}
//	}
//	return nil
//}
//func makeRafts() []RaftNode{
//	return nil
//}
