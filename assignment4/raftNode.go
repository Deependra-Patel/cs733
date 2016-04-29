package main

import (
	"encoding/gob"
	//"fmt"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/log"
	"io/ioutil"
	logger "log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
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
	rn.sm = &StateMachine{id: config.Id, term: 0, commitIndex: 0, state: "Follower",
		peers: peerIds, votedFor: 0, log: make([]logEntry, 0), voteCount: 0,
		netCh: make(chan interface{}), timeoutCh: make(chan interface{}), actionCh: make(chan interface{}),
		clientCh: make(chan interface{}), matchIndex: map[int]int{},
		nextIndex: map[int]int{}, leaderId: -1, HeartbeatTimeout: config.HeartbeatTimeout,
		ElectionTimeout: config.ElectionTimeout}

	//stateStore file
	rn.stateStoreFile = "stateStoreFile" + strconv.Itoa(config.Id)
	contents, err := ioutil.ReadFile(rn.stateStoreFile)
	if len(contents) != 0 {
		state_VotedFor := strings.Split(string(contents), " ")
		term, err1 := strconv.Atoi(state_VotedFor[0])
		votedFor, err2 := strconv.Atoi(state_VotedFor[1])
		if err1 != nil || err2 != nil {
			logger.Panic("Can't convert term/votedFor to int")
		} else {
			rn.sm.votedFor = votedFor
			rn.sm.term = term
		}
	} else {
		ioutil.WriteFile(rn.stateStoreFile, []byte("0 0"), 0777)
	}

	//setting log
	for _, peerId := range peerIds {
		rn.sm.matchIndex[peerId] = 0
		rn.sm.nextIndex[peerId] = 1
	}
	lg, err := log.Open(config.LogDir)
	if err != nil {
		//logger.Println("Log can't be opened/created", err)
	}
	rn.lg = lg
	rn.lg.RegisterSampleEntry(logEntry{})
	lastIndex := int(rn.lg.GetLastIndex())
	if lastIndex != -1 {
		for i := 0; i <= lastIndex; i++ {
			data, err := rn.lg.Get(int64(i))
			//fmt.Println(data, err)
			if err != nil {
				logger.Panic("Read from log not possible")
			}
			rn.sm.log = append(rn.sm.log, data.(logEntry))
		}
	} else {
		err = rn.lg.Append(logEntry{Term: 0, Data: []byte("Dummy")})
		rn.sm.log = append(rn.sm.log, logEntry{Term: 0, Data: []byte("Dummy")})
		if err != nil {
			//logger.Println("Couldn't write to log", err)
		}
	}

	//configuring server
	serverConfig := createServerConfig(config.cluster)
	rn.server, err = cluster.New(config.Id, serverConfig)
	if err != nil {
		logger.Panic("Couldn't start cluster server", err)
	} else {
		//logger.Println("ID:"+strconv.Itoa(config.Id)+" Raft Server Started succesfully")
	}
	gob.Register(AppendEntriesReqEv{})
	gob.Register(AppendEntriesRespEv{})
	gob.Register(VoteReqEv{})
	gob.Register(VoteRespEv{})

	//setting channels
	rn.timeoutChan = make(chan interface{})
	rn.commitChan = make(chan CommitInfo)
	rn.eventChan = make(chan interface{})

	rn.lock = &sync.Mutex{}

	//configuring timer
	timerFunc := func(rn *RaftNode) func() {
		return func() {
			rn.timeoutChan <- TimeoutEv{}
		}
	}(&rn)
	rn.timer = time.AfterFunc(rn.sm.ElectionTimeout, timerFunc)
	//logger.Println("Created new raft node")
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
	//logger.Println("Append request received")
	rn.eventChan <- AppendEv{b}
}

// A channel for client to listen on. What goes into Append must come out of here at some point.
func (rn *RaftNode) CommitChannel() <-chan CommitInfo {
	return rn.commitChan
}

// Last known committed index in the log.
func (rn *RaftNode) CommittedIndex() int {
	//This could be -1 until the system stabilizes.
	rn.lock.Lock()
	commitIndex := rn.sm.commitIndex
	rn.lock.Unlock()
	return commitIndex
}

// Returns the data at a log index, or an error.
func (rn *RaftNode) Get(index int) (error, []byte) {
	itf, err := rn.lg.Get(int64(index))
	if err != nil {
		return err, nil
	}
	return err, (itf.(logEntry)).Data
}

// Node's id
func (rn *RaftNode) Id() int {
	return rn.sm.id
}

// Id of leader. -1 if unknown
func (rn *RaftNode) LeaderId() int {
	rn.lock.Lock()
	leaderId := rn.sm.leaderId
	rn.lock.Unlock()
	return leaderId
}

// Signal to shut down all goroutines, stop sockets, flush log and close it, cancel timers.
func (rn *RaftNode) Shutdown() {
	rn.eventChan <- shutdownEvent{}
	rn.timer.Stop()
	rn.lg.Close()
	rn.server.Close()
	//logger.Println("Succesfully shutdown ID:", rn.Id())
}

// Deletes log folder and the stateStore file
func (rn *RaftNode) Delete() {
	err1 := os.RemoveAll("mylog" + strconv.Itoa(rn.Id()))
	err2 := os.Remove("stateStoreFile" + strconv.Itoa(rn.Id()))
	if err1 != nil || err2 != nil {
		logger.Panic("Couldn't delete files/folder", err1, err2)
	}
}

func (rn *RaftNode) doActions(actions []interface{}) {
	for _, action := range actions {
		switch action.(type) {
		case Alarm:
			alarm := action.(Alarm)
			timerFunc := func(rn *RaftNode) func() {
				return func() {
					rn.timeoutChan <- TimeoutEv{}
				}
			}(rn)
			rn.timer.Stop()
			rn.timer = time.AfterFunc(alarm.t, timerFunc)
		//logger.Printf("ID:%v Setting Alarm.. ", rn.Id(), alarm.t.Nanoseconds())

		case LogStore:
			logStore := action.(LogStore)
			//logger.Printf("ID:%v Storing in Log %+v\n", rn.Id(), logStore)
			rn.lg.TruncateToEnd(int64(logStore.index))
			rn.lg.Append(logEntry{Term: logStore.term, Data: logStore.data})
		case StateStore:
			stateStore := action.(StateStore)
			//logger.Printf("ID:%v Storing current term, votedfor.. %+v\n", rn.Id(), stateStore)
			err := ioutil.WriteFile(rn.stateStoreFile, []byte(strconv.Itoa(stateStore.currentTerm)+" "+
			strconv.Itoa(stateStore.votedFor)), 0777)
			if err != nil {
				logger.Panic("Can't write to stateStore file", err)
			}
		case Send:
			send := action.(Send)
			//logger.Printf("ID:%v Sending.. %+v\n", rn.Id(), action)
			switch send.event.(type) {
			case VoteReqEv:
				voteReq := send.event.(VoteReqEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: voteReq}
			case VoteRespEv:
				voteResp := send.event.(VoteRespEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: voteResp}
			case AppendEntriesReqEv:
				appendEntrReq := send.event.(AppendEntriesReqEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: appendEntrReq}
			case AppendEntriesRespEv:
				appendEntrRes := send.event.(AppendEntriesRespEv)
				rn.server.Outbox() <- &cluster.Envelope{Pid: send.peerId, Msg: appendEntrRes}
			}
		case Commit:
			commit := action.(Commit)
			//logger.Printf("ID:%v Committing.. %+v", rn.Id(), commit)
			rn.commitChan <- CommitInfo{data: commit.data, err: commit.err, Index: commit.index}
		default:
			println("Unrecognized")
		}
	}
}

func getActionsFromSM(rn *RaftNode, event interface{}) []interface{} {
	rn.lock.Lock()
	actions := rn.sm.ProcessEvent(event)
	rn.lock.Unlock()
	return actions
}

func (rn *RaftNode) processEvents() {
	infiLoop:
	for {
		select {
		case <-rn.timeoutChan:
		//logger.Printf("ID:%v Timeout\n", rn.Id())
			rn.doActions(getActionsFromSM(rn, TimeoutEv{}))
		case ev := <-rn.eventChan:
			switch ev.(type) {
			case shutdownEvent:
				break infiLoop
			default:
				//logger.Printf("ID:%v Append %+v\n", rn.Id(), ev)
				rn.doActions(getActionsFromSM(rn, ev))
			}
		case inbox := <-rn.server.Inbox():
		//logger.Printf("ID:%v Inbox %+v\n", rn.Id(), inbox)
			rn.doActions(getActionsFromSM(rn, inbox.Msg))
		}
	}
}