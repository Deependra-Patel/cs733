package main
import "fmt"

type Timeout struct {
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

func (sm *StateMachine) voteReq(voteReq VoteReqEv) []interface{}{

	var resp []interface{}
	switch sm.state {
	case "Follower":
		if sm.term <= voteReq.term &&
		(sm.votedFor == 0 || sm.votedFor == voteReq.candidateId) &&
		(voteReq.term > sm.log[len(sm.log)-1].term || ((voteReq.term == sm.log[len(sm.log)-1].term )&& voteReq.lastLogIndex >= len(sm.log))){
			sm.term = voteReq.term
			sm.votedFor = voteReq.candidateId
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, true}})
			resp = append(resp, StateStore{sm.term, sm.votedFor})
		} else {
			resp = append(resp, VoteRespEv{sm.term, false})
		}
	case "Candidate":
		if sm.term < voteReq.term &&
		(voteReq.term > sm.log[len(sm.log)-1].term || ((voteReq.term == sm.log[len(sm.log)-1].term )&& voteReq.lastLogIndex >= len(sm.log))){
			sm.state = "Follower"
			sm.term = voteReq.term
			sm.votedFor = voteReq.candidateId
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, true}})
			resp = append(resp, StateStore{sm.term, sm.votedFor})
		} else {
			resp = append(resp, VoteRespEv{sm.term, false})
		}
	case "Leader":
		if sm.term >= voteReq.term{
			resp = append(resp, Send{voteReq.candidateId, VoteRespEv{sm.term, false}})
			//resp = append(resp, Send{voteReq.candidateId, AppendEntriesReqEv{sm.term, sm.id, len(sm.log), sm.log[len(sm.log)-1], nil, sm.commitIndex}})
		}

	}
	return resp
}


func main() {
	// testing testing
	var sm StateMachine
	sm.ProcessEvent(AppendEntriesReqEv{term : 10, prevLogIndex: 100, prevLogTerm: 3})
//	sm.ProcessEvent(VoteReqEv{term : 10, candidateId: 100})
}

func (sm *StateMachine) ProcessEvent (ev interface{}) {
	switch ev.(type) {
	case AppendEntriesReqEv:
		cmd := ev.(AppendEntriesReqEv)
		// do stuff with req
		fmt.Printf("%v\n", cmd)
	case VoteReqEv:
		cmd := ev.(VoteReqEv)
		fmt.Printf("%v\n", cmd.candidateId)

	// other cases
	default: println("Unrecognized")
	}
}

