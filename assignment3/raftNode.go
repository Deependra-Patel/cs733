package assignment3

import (
	"github.com/cs733-iitb/log"
	"fmt"
)

// Returns a Node object
func raftNew(config Config) *RaftNode{
	peerIds := make([]int, len(config.cluster)-1)
	for i, netconfig := range config.cluster{
		if (netconfig.Id != config.Id) {
			peerIds[i] = netconfig.Id
		}
	}
	rn := &RaftNode{}
	rn.sm = StateMachine{id: config.Id, term: 0, commitIndex: 0, state: "Follower",
			peers: peerIds, votedFor: 0, log: make([]logEntry, 0), voteCount: 0,
			netCh: make(chan interface{}), timeoutCh: make(chan interface{}), actionCh: make(chan interface{}),
			clientCh: make(chan interface{}), matchIndex: map[int]int{},
			nextIndex: map[int]int{}}
	lg, err := log.Open(config.LogDir)
	if err!=nil{
		fmt.Errorf("Log can't be created")
	}
	rn.lg = lg
	go func() {
		rn.sm.ProcessEvent()
	}()
	return rn
}


func (rn *RaftNode) Append(b []byte){
	rn.sm.clientCh <- b
}

// A channel for client to listen on. What goes into Append must come out of here at some point.
func (rn *RaftNode) CommitChannel() <- chan CommitInfo{

}

// Last known committed index in the log.
func (rn *RaftNode) CommittedIndex() int {
	//This could be -1 until the system stabilizes.
	//return rn.lg
}

// Returns the data at a log index, or an error.
func (rn *RaftNode) Get(index int) (error, []byte){
	return rn.lg.Get(index)
}

// Node's id
func (rn *RaftNode) Id(){
	return rn.sm.id
}

// Id of leader. -1 if unknown
func (rn *RaftNode) LeaderId() int{
	return rn.sm.leaderId
}

// Signal to shut down all goroutines, stop sockets, flush log and close it, cancel timers.
func (rn *RaftNode) Shutdown(){
	rn.lg.Close()
}
