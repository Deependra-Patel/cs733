package main
import (
	"fmt"
)

var timeoutTime int = 10
type Timeout struct {
}

type Alarm struct {
	t int
}

type logEntry struct{
	term int
	cmd string
}

type AppendEntriesReqEv struct {
	term int
	leaderId int
	prevLogIndex int
	prevLogTerm int
	entries []logEntry
	leaderCommit int
}

type AppendEntriesRespEv struct{
	term int
	success bool
}

type VoteReqEv struct{
	term int
	candidateId int
	lastLogIndex int
	lastLogTerm int
}

type VoteRespEv struct{
	term int
	voteGranted bool
}

type StateMachine struct {
	id int // server id
	state string
	peers []int // other server ids
	term int
	voteCount int
	log []logEntry
	votedFor int
	commitIndex int
	nextIndex map[int]int
	matchIndex map[int]int
}

type Send struct{
	peerId int
	event interface{}
}

type StateStore struct {
	currentTerm int
	votedFor int
}
func (alarm *Alarm) print(){
	fmt.Printf("Alarm %+v\n", *alarm)
}
func (ss *StateStore) print(){
	fmt.Printf("StateStore %+v\n", *ss)
}

func (send *Send) print(){
	fmt.Printf("Send {peerId:%d ", send.peerId)
	switch (send.event).(type){
	case VoteRespEv:
		ev := (send.event).(VoteRespEv)
		fmt.Printf("VoteRespEv %+v", ev)
	case VoteReqEv:
		ev := (send.event).(VoteReqEv)
		fmt.Printf("VoteReqEv %+v", ev)
	case AppendEntriesReqEv:
		ev := (send.event).(AppendEntriesReqEv)
		fmt.Printf("AppendEntriesReqEv %+v", ev)
	case AppendEntriesRespEv:
		ev := (send.event).(AppendEntriesRespEv)
		fmt.Printf("AppendEntriesRespEv %+v", ev)
	}
	fmt.Println("}")
}


func (sm *StateMachine) voteReq(voteReq VoteReqEv) []interface{}{
	var resp []interface{}
	switch sm.state {
	case "Follower":
		if sm.term <= voteReq.term &&
		(sm.votedFor == 0 || sm.votedFor == voteReq.candidateId) &&
		(voteReq.term > sm.log[len(sm.log)-1].term || ((voteReq.term == sm.log[len(sm.log)-1].term )&& voteReq.lastLogIndex >= len(sm.log))){
			sm.term = voteReq.term
			sm.votedFor = voteReq.candidateId
			resp = append(resp, StateStore{sm.term, sm.votedFor})
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, true}})
		} else {
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, false}})
		}
	case "Candidate":
		if sm.term < voteReq.term {
			sm.state = "Follower"
			sm.term = voteReq.term
			if (voteReq.term > sm.log[len(sm.log)-1].term || ((voteReq.term == sm.log[len(sm.log)-1].term )&& voteReq.lastLogIndex >= len(sm.log))){
				sm.votedFor = voteReq.candidateId
				resp = append(resp, StateStore{sm.term, sm.votedFor})
				resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, true}})
			} else {
				resp = append(resp, StateStore{sm.term, 0})
				resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, false}})
			}
		} else {
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, false}})
		}
	case "Leader":
		if sm.term >= voteReq.term{
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, false}})
			resp = append(resp, Send{voteReq.candidateId, AppendEntriesReqEv{sm.term, sm.id, len(sm.log)-1, sm.log[len(sm.log)-1].term, nil, sm.commitIndex}})
		} else {
			sm.state = "Follower"
			sm.term = voteReq.term
			if (voteReq.term > sm.log[len(sm.log)-1].term || ((voteReq.term == sm.log[len(sm.log)-1].term )&& voteReq.lastLogIndex >= len(sm.log))){
				sm.votedFor = voteReq.candidateId
				resp = append(resp, StateStore{sm.term, sm.votedFor})
				resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, true}})
			} else { //reject
				resp = append(resp, StateStore{sm.term, 0})
				resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, false}})
			}
		}

	}
	return resp
}

func (sm *StateMachine) voteResp(voteResp VoteRespEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
	case "Candidate":
		if voteResp.term == sm.term && voteResp.voteGranted{
			sm.voteCount++
			if sm.voteCount >= (len(sm.peers)+3)/2{
				sm.state = "Leader"
				resp = append(resp, getHeartBeatEvents(sm)...)
				for _, peer := range sm.peers{
					sm.nextIndex[peer] = len(sm.log)+1
					sm.matchIndex[peer] = 0
				}
			}
		}
	case "Leader":
	}
	return resp
}

func (sm *StateMachine) timeout() []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
		sm.state = "Candidate"
		sm.term++
		sm.votedFor = sm.id
		sm.voteCount = 1
		resp = append(resp, StateStore{sm.term, sm.votedFor})
		resp = append(resp, Alarm{timeoutTime})
		for _, peer := range sm.peers{
			resp = append(resp, Send{peer, VoteReqEv{term:sm.term, candidateId:sm.id,
				lastLogIndex:len(sm.log), lastLogTerm:sm.log[len(sm.log)-1].term}})
		}
	case "Candidate":
		sm.term ++
		sm.voteCount = 1
		sm.votedFor = sm.id
		resp = append(resp, StateStore{sm.term, sm.votedFor})
		resp = append(resp, Alarm{timeoutTime})
		for _, peer := range sm.peers{
			resp = append(resp, Send{peer, VoteReqEv{term:sm.term, candidateId:sm.id,
				lastLogIndex:len(sm.log), lastLogTerm:sm.log[len(sm.log)-1].term}})
		}
	case "Leader":
		resp = append(resp, Alarm{timeoutTime})
		resp = append(resp, getHeartBeatEvents(sm)...)
	}
	return resp
}

func getHeartBeatEvents(sm *StateMachine) []interface{} {
	var resp []interface{}
	for _, peer := range sm.peers{
		resp = append(resp, Send{peer, AppendEntriesReqEv{sm.term, sm.id, len(sm.log) - 1, sm.log[len(sm.log) - 1].term, nil, sm.commitIndex}})
	}
	return resp
}

func main() {
	// testing testing
	sampleLog := []logEntry{logEntry{0,"ff"}, logEntry{1,"dd"}, logEntry{1, "jj"}}
	sm := StateMachine{id:3, term:3, commitIndex:1, state:"Follower", votedFor:0, log:sampleLog}
	//sm.ProcessEvent(AppendEntriesReqEv{term : 10, prevLogIndex: 100, prevLogTerm: 3})
	sm.ProcessEvent(VoteReqEv{term : 2, candidateId: 1, lastLogIndex:3, lastLogTerm:1})

	sm = StateMachine{id:3, term:3, commitIndex:1, state:"Leader", votedFor:0, log:sampleLog}
	sm.ProcessEvent(VoteReqEv{term : 4, candidateId: 1, lastLogIndex:3, lastLogTerm:3})

	sm = StateMachine{id:3, term:4, commitIndex:1, peers:[]int{2,5,6},
		matchIndex:make(map[int]int), nextIndex:make(map[int]int), state:"Follower", votedFor:0, log:sampleLog}
	sm.ProcessEvent(VoteRespEv{term : 4, voteGranted:true})
	sm.ProcessEvent(VoteRespEv{term : 4, voteGranted:true})
	sm.ProcessEvent(VoteRespEv{term : 4, voteGranted:true})
	sm.ProcessEvent(Timeout{})
}

func (sm *StateMachine) ProcessEvent (ev interface{}) {
	var outputEvents []interface{}
	switch ev.(type) {
	case AppendEntriesReqEv:
		event := ev.(AppendEntriesReqEv)
		print(event.term)
		//outputEvents = sm.Ap
	case VoteReqEv:
		event := ev.(VoteReqEv)
		outputEvents = sm.voteReq(event)
	case VoteRespEv:
		event := ev.(VoteRespEv)
		outputEvents = sm.voteResp(event)
	case Timeout:
		outputEvents = sm.timeout()
	// other cases
	default: println("Unrecognized")
	}
	fmt.Println("No. of events ", len(outputEvents))
	for _, element := range outputEvents{
		switch element.(type){
		case StateStore:
			ev := element.(StateStore)
			ev.print()
		case Send:
			ev := element.(Send)
			ev.print()
		case Alarm:
			ev := element.(Alarm)
			ev.print()
		}
	}
}

