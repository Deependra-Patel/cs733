package main

import (
	"encoding/gob"
	"fmt"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/log"
	"io/ioutil"
	logger "log"
	"os"
	"strconv"
)

// Returns a Node object
func New(config Config) RaftNode {
	peerIds := make([]int, 0)
	for _, netconfig := range config.cluster {
		if netconfig.Id != config.Id {
			peerIds = append(peerIds, netconfig.Id)
		}
	}
	rn := RaftNode{}
	rn.sm = StateMachine{id: config.Id, term: 0, commitIndex: 0, state: "Follower",
		peers: peerIds, votedFor: 0, log: make([]logEntry, 0), voteCount: 0,
		netCh: make(chan interface{}), timeoutCh: make(chan interface{}), actionCh: make(chan interface{}),
		clientCh: make(chan interface{}), matchIndex: map[int]int{},
		nextIndex: map[int]int{}}
	lg, err := log.Open(config.LogDir)
	if err != nil {
		fmt.Errorf("Log can't be created")
	}
	rn.lg = lg
	rn.lg.RegisterSampleEntry(logEntry{})
	serverConfig := createServerConfig(config.cluster)
	rn.server, err = cluster.New(config.Id, serverConfig)
	if err != nil {
		logger.Panic("Couldn't start cluster server", err)
	} else {
		logger.Println("Server Started succesfully")
	}
	gob.Register(AppendEntriesReqEv{})
	gob.Register(AppendEntriesRespEv{})
	gob.Register(VoteReqEv{})
	gob.Register(VoteRespEv{})
	go func() {
		rn.sm.eventLoop()
	}()
	rn.commitChan = make(chan CommitInfo)
	rn.stateStoreFile = "stateStoreFile"
	_, err = os.Create(rn.stateStoreFile)
	if err != nil {
		logger.Panic("Couldn't create stateStoreFile", err)
	} else {
		logger.Println("stateStoreFile created succesfully")
	}
	return rn
}

func createServerConfig(netConfigs []NetConfig) cluster.Config {
	peerConfigs := make([]cluster.PeerConfig, len(netConfigs))
	for i, netConfig := range netConfigs {
		peerConfigs[i] = cluster.PeerConfig{Id: netConfig.Id, Address: netConfig.Host +
			":" + strconv.Itoa(netConfig.Port)}
	}
	return cluster.Config{
		Peers: peerConfigs,
	}
}

func (rn *RaftNode) Append(b []byte) {
	logger.Println("Append request received")
	rn.eventChan <- AppendEv{b}
}

// A channel for client to listen on. What goes into Append must come out of here at some point.
func (rn *RaftNode) CommitChannel() <-chan CommitInfo {
	return rn.commitChan
}

// Last known committed index in the log.
func (rn *RaftNode) CommittedIndex() int {
	//This could be -1 until the system stabilizes.
	return rn.sm.commitIndex
}

// Returns the data at a log index, or an error.
func (rn *RaftNode) Get(index int) (error, []byte) {
	itf, err := rn.lg.Get(int64(index))
	return err, (itf.(logEntry)).data
}

// Node's id
func (rn *RaftNode) Id() int {
	return rn.sm.id
}

// Id of leader. -1 if unknown
func (rn *RaftNode) LeaderId() int {
	return rn.sm.leaderId
}

// Signal to shut down all goroutines, stop sockets, flush log and close it, cancel timers.
func (rn *RaftNode) Shutdown() {
	rn.lg.Close()
	rn.server.Close()
}

func (rn *RaftNode) doActions(actions []interface{}) {
	for _, action := range actions {
		switch action.(type) {
		//case Alarm:
		//	timer := action.(Alarm)
		case LogStore:
			logStore := action.(LogStore)
			rn.lg.Append(logEntry{term: logStore.term, data: logStore.data})
		case StateStore:
			stateStore := action.(StateStore)
			ioutil.WriteFile(rn.stateStoreFile, []byte(string(stateStore.currentTerm)+" "+
				string(stateStore.votedFor)), 0644)
		case Send:
			send := action.(Send)
			switch send.event.(type) {
			case VoteReqEv:
				voteReq := send.event.(VoteReqEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: voteReq}
			case VoteRespEv:
				voteResp := send.event.(VoteRespEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: voteResp}
			case AppendEntriesReqEv:
				appendEntrReq := send.event.(VoteReqEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: appendEntrReq}
			case AppendEntriesRespEv:
				appendEntrRes := send.event.(VoteReqEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: appendEntrRes}
			}
		default:
			println("Unrecognized")
		}
	}
}

func getActionsFromSM(rn *RaftNode) []interface{} {
	var actions []interface{}
	for {
		select {
		case action := <-rn.sm.actionCh:
			switch action.(type) {
			case Finish:
			default:
				actions = append(actions, action)
			}
		}
	}
	return actions
}

func (rn *RaftNode) processEvents() {
	for {
		select {
		case ev := <- rn.eventChan:
			fmt.Println("hiii")
			rn.sm.clientCh <- ev.(AppendEv)
			logger.Println("Received this", ev)
			rn.doActions(getActionsFromSM(rn))
		case inbox := <-rn.server.Inbox():
			logger.Println("Received this", inbox)
			rn.doActions(getActionsFromSM(rn))
		case <-rn.timeoutChan:
			logger.Println("Timeout occured on node")
			rn.sm.timeoutCh <- TimeoutEv{}
		}
	}
}

func main() {
	logger.SetFlags(logger.LstdFlags | logger.Lshortfile)
	r1 := New(
		Config{
			cluster: []NetConfig{
				{Id: 1, Host: "localhost", Port: 7000},
				{Id: 2, Host: "localhost", Port: 7001},
				{Id: 3, Host: "localhost", Port: 7002},
			},
			Id:               1,
			LogDir:           "mylog",
			ElectionTimeout:  2,
			HeartbeatTimeout: 1,
		})
	r1.sm.state = "Leader"
	go func() {
		r1.processEvents()
	}()
	fmt.Println("testing")
	//r1.Append([]byte("hi deependra"))
	r1.timeoutChan <- timeoutTime
	fmt.Println("sent")
}
