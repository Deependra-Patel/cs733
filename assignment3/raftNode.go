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
	"time"
	"sync"
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
		peers: peerIds, votedFor: 0, log: make([]logEntry, 1), voteCount: 0,
		netCh: make(chan interface{}), timeoutCh: make(chan interface{}), actionCh: make(chan interface{}),
		clientCh: make(chan interface{}), matchIndex: map[int]int{},
		nextIndex: map[int]int{}, leaderId:-1, HeartbeatTimeout:config.HeartbeatTimeout,
		ElectionTimeout:config.ElectionTimeout}

	rn.sm.log[0] = logEntry{Term:0, Data:[]byte("Dummy")}
	for _, peerId := range peerIds{
		rn.sm.matchIndex[peerId] = 0
		rn.sm.nextIndex[peerId] = 1
	}
	lg, err := log.Open(config.LogDir)
	lg.TruncateToEnd(0)
	if err != nil {
		logger.Println("Log can't be opened/created", err)
	}
	rn.lg = lg
	rn.lg.RegisterSampleEntry(logEntry{})
	err = rn.lg.Append(logEntry{Term:0, Data:[]byte("Dummy")})
	if err != nil{
		logger.Println("Couldn't write to log", err)
	}
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

	rn.timeoutChan = make(chan interface{})
	rn.commitChan = make(chan CommitInfo)
	rn.eventChan = make(chan interface{})

	rn.lock = &sync.Mutex{}

	rn.stateStoreFile = "stateStoreFile" + strconv.Itoa(config.Id)
	_, err = os.Create(rn.stateStoreFile)
	if err != nil {
		logger.Panic("Couldn't create stateStoreFile", err)
	} else {
		logger.Println("stateStoreFile created succesfully")
	}

	timerFunc := func(rn *RaftNode) func(){
		return func() {
			rn.timeoutChan <- TimeoutEv{}
		}
	}(&rn)

	rn.timer = time.AfterFunc(rn.sm.ElectionTimeout, timerFunc)
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
	logger.Println("Succesfully shutdown ID:", rn.Id())
}

func (rn *RaftNode) doActions(actions []interface{}) {
	for _, action := range actions {
		switch action.(type) {
		case Alarm:
			alarm := action.(Alarm)
			logger.Println("Setting Alarm.. ID:", rn.Id(), alarm.t.Nanoseconds())
			timerFunc := func(rn *RaftNode) func(){
				return func() {
					logger.Println("Sending to timeoutChan ID: ", rn.Id())
					rn.timeoutChan <- TimeoutEv{}
				}
			}(rn)
			rn.timer.Stop()
			rn.timer = time.AfterFunc(alarm.t, timerFunc)

		case LogStore:
			logStore := action.(LogStore)
			logger.Printf("Storing in Log ID:%v %+v\n", rn.Id(), logStore)
			rn.lg.TruncateToEnd(int64(logStore.index))
			rn.lg.Append(logEntry{Term: logStore.term, Data: logStore.data})
		case StateStore:
			stateStore := action.(StateStore)
			ioutil.WriteFile(rn.stateStoreFile, []byte(string(stateStore.currentTerm)+" "+
				string(stateStore.votedFor)), 0644)
		case Send:
			send := action.(Send)
			logger.Printf("Sending.. %+v\n", action)
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
			logger.Println(rn.sm.state)
			logger.Println("Committing.. ID:", rn.Id())
			rn.commitChan <- CommitInfo{data:commit.data, err:commit.err, Index:commit.index}
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
		case ev := <- rn.eventChan:
			switch ev.(type) {
			case shutdownEvent:
				break infiLoop
			default:
				logger.Printf("ID:%v Append %+v\n", rn.Id(), ev)
				rn.doActions(getActionsFromSM(rn, ev))
			}
		case inbox := <-rn.server.Inbox():
			logger.Printf("ID:%v Inbox %+v\n", rn.Id(), inbox)
			rn.doActions(getActionsFromSM(rn, inbox.Msg))
		case <-rn.timeoutChan:
			logger.Printf("ID:%v Timeout\n", rn.Id())
			rn.doActions(getActionsFromSM(rn, TimeoutEv{}))
		}
	}
}

func main() {
	logger.SetFlags(logger.LstdFlags | logger.Lshortfile)
	r1 := New(
		Config{
			cluster: []NetConfig{
				{Id: 1, Host: "localhost", Port: 7000},
				{Id: 4, Host: "localhost", Port: 7001},
				{Id: 6, Host: "localhost", Port: 7002},
			},
			Id:               1,
			LogDir:           "mylog1",
			ElectionTimeout:  time.Millisecond*time.Duration(300),
			HeartbeatTimeout: time.Millisecond*time.Duration(100),
		})
	r2 := New(
		Config{
			cluster: []NetConfig{
				{Id: 1, Host: "localhost", Port: 7000},
				{Id: 4, Host: "localhost", Port: 7001},
				{Id: 6, Host: "localhost", Port: 7002},
			},
			Id:               4,
			LogDir:           "mylog2",
			ElectionTimeout:  time.Millisecond*time.Duration(300),
			HeartbeatTimeout: time.Millisecond*time.Duration(100),
		})
	//r3 := New(
	//	Config{
	//		cluster: []NetConfig{
	//			{Id: 1, Host: "localhost", Port: 7000},
	//			{Id: 4, Host: "localhost", Port: 7001},
	//			{Id: 6, Host: "localhost", Port: 7002},
	//		},
	//		Id:               6,
	//		LogDir:           "mylog",
	//		ElectionTimeout:  2,
	//		HeartbeatTimeout: 1,
	//	})

	go func() {
		r1.processEvents()
	}()
	go func() {
		r2.processEvents()
	}()
	//time.Sleep(time.Millisecond*time.Duration(100))
	//go func() {
	//	r3.processEvents()
	//}()
	r1.Append([]byte("hi deependra"))
	commitInfo := <- r1.commitChan
	logger.Println(commitInfo)
	time.Sleep(time.Millisecond*time.Duration(500))
	r1.Append([]byte("hi deependra"))
	commitInfo = <- r1.commitChan
	logger.Println(commitInfo)
	err, bt := r1.Get(2)
	fmt.Println("-----------------------", err, string(bt));
	//time.Sleep(time.Millisecond*time.Duration(1000))
	fmt.Println("sent")
}
