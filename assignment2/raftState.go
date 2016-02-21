package assignment2

import (
	"fmt"
)

var timeoutTime int = 10
var DEBUG = false

func (commit *Commit) print() {
	fmt.Printf("Commit %+v\n", *commit)
}

func (alarm *Alarm) print() {
	fmt.Printf("Alarm %+v\n", *alarm)
}
func (ss *StateStore) print() {
	fmt.Printf("StateStore %+v\n", *ss)
}
func (ls *LogStore) print() {
	fmt.Printf("LogStore %+v\n", *ls)
}

func (send *Send) print() {
	fmt.Printf("Send {peerId:%d ", send.peerId)
	switch (send.event).(type) {
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

func printOutputActions(outputActions []interface{}) {
	fmt.Println("No. of output actions ", len(outputActions))
	for _, element := range outputActions {
		switch element.(type) {
		case Send:
			ev := element.(Send)
			ev.print()
		case Commit:
			ev := element.(Commit)
			ev.print()
		case Alarm:
			ev := element.(Alarm)
			ev.print()
		case LogStore:
			ev := element.(LogStore)
			ev.print()
		case StateStore:
			ev := element.(StateStore)
			ev.print()
		}
	}
	fmt.Println()
}

func (sm *StateMachine) voteReq(voteReq VoteReqEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
		if sm.term <= voteReq.term &&
			(sm.votedFor == 0 || sm.votedFor == voteReq.candidateId) &&
			(voteReq.term > sm.log[len(sm.log)-1].term ||
				((voteReq.term == sm.log[len(sm.log)-1].term) && voteReq.lastLogIndex >= (len(sm.log)-1))) {
			sm.term = voteReq.term
			sm.votedFor = voteReq.candidateId
			resp = append(resp, StateStore{sm.term, sm.votedFor})
			resp = append(resp, Send{voteReq.candidateId,
				VoteRespEv{term: sm.term, voteGranted: true, from: sm.id}})
		} else {
			resp = append(resp, Send{voteReq.candidateId,
				VoteRespEv{term: sm.term, voteGranted: false, from: sm.id}})
		}
	case "Candidate":
		if sm.term < voteReq.term {
			sm.state = "Follower"
			sm.term = voteReq.term
			if voteReq.term > sm.log[len(sm.log)-1].term ||
				((voteReq.term == sm.log[len(sm.log)-1].term) && voteReq.lastLogIndex >= (len(sm.log)-1)) {
				sm.votedFor = voteReq.candidateId
				resp = append(resp, StateStore{sm.term, sm.votedFor})
				resp = append(resp, Send{voteReq.candidateId,
					VoteRespEv{term: sm.term, voteGranted: true, from: sm.id}})
			} else {
				resp = append(resp, StateStore{sm.term, 0})
				resp = append(resp, Send{voteReq.candidateId,
					VoteRespEv{term: sm.term, voteGranted: false, from: sm.id}})
			}
		} else {
			resp = append(resp, Send{voteReq.candidateId,
				VoteRespEv{term: sm.term, voteGranted: false, from: sm.id}})
		}
	case "Leader":
		if sm.term >= voteReq.term {
			resp = append(resp, Send{voteReq.candidateId,
				VoteRespEv{term: sm.term, voteGranted: false, from: sm.id}})
			resp = append(resp, Send{voteReq.candidateId, AppendEntriesReqEv{sm.term,
				sm.id, len(sm.log) - 1, sm.log[len(sm.log)-1].term, nil, sm.commitIndex}})
		} else {
			sm.state = "Follower"
			sm.term = voteReq.term
			if voteReq.term > sm.log[len(sm.log)-1].term ||
				((voteReq.term == sm.log[len(sm.log)-1].term) && voteReq.lastLogIndex >= (len(sm.log)-1)) {
				sm.votedFor = voteReq.candidateId
				resp = append(resp, StateStore{sm.term, sm.votedFor})
				resp = append(resp, Send{voteReq.candidateId, VoteRespEv{term: sm.term,
					voteGranted: true, from: sm.id}})
			} else { //reject
				resp = append(resp, StateStore{sm.term, 0})
				resp = append(resp, Send{voteReq.candidateId, VoteRespEv{term: sm.term,
					voteGranted: false, from: sm.id}})
			}
		}

	}
	return resp
}

func (sm *StateMachine) voteResp(voteResp VoteRespEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
		if sm.term <= voteResp.term {
			println("ERROR: Inconsistent Input")
		}
	case "Candidate":
		if voteResp.term == sm.term && voteResp.voteGranted {
			sm.voteCount++
			if sm.voteCount >= (len(sm.peers)+3)/2 {
				sm.state = "Leader"
				resp = append(resp, Alarm{t: timeoutTime})
				resp = append(resp, getHeartBeatEvents(sm)...)
				for _, peer := range sm.peers {
					sm.nextIndex[peer] = len(sm.log)
					sm.matchIndex[peer] = 0
				}
			}
		}
	case "Leader":
		if sm.term < voteResp.term {
			println("ERROR: Inconsistent Input")
		}
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
		for _, peer := range sm.peers {
			resp = append(resp, Send{peer, VoteReqEv{term: sm.term, candidateId: sm.id,
				lastLogIndex: len(sm.log) - 1, lastLogTerm: sm.log[len(sm.log)-1].term}})
		}
	case "Candidate":
		sm.term++
		sm.voteCount = 1
		sm.votedFor = sm.id
		resp = append(resp, StateStore{sm.term, sm.votedFor})
		resp = append(resp, Alarm{timeoutTime})
		for _, peer := range sm.peers {
			resp = append(resp, Send{peer, VoteReqEv{term: sm.term, candidateId: sm.id,
				lastLogIndex: len(sm.log) - 1, lastLogTerm: sm.log[len(sm.log)-1].term}})
		}
	case "Leader":
		resp = append(resp, Alarm{timeoutTime})
		resp = append(resp, getHeartBeatEvents(sm)...)
	}
	return resp
}

func (sm *StateMachine) appendEntriesReq(appendEntries AppendEntriesReqEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
		if appendEntries.term < sm.term {
			resp = append(resp, Send{appendEntries.leaderId,
				AppendEntriesRespEv{from: sm.id, term: sm.term, success: false}})
		} else {
			if appendEntries.term > sm.term {
				sm.term = appendEntries.term
				resp = append(resp, StateStore{currentTerm: sm.term, votedFor: 0})
			}
			resp = append(resp, Alarm{t: timeoutTime})
			if len(sm.log) > appendEntries.prevLogIndex &&
				sm.log[appendEntries.prevLogIndex].term == appendEntries.prevLogTerm {
				sm.log = append(sm.log[:appendEntries.prevLogIndex+1], appendEntries.entries...)
				index := appendEntries.prevLogIndex + 1
				i := 0
				for i < len(appendEntries.entries) {
					resp = append(resp, LogStore{index: index + i, term: sm.term,
						data: appendEntries.entries[i].data})
					i += 1
				}
				resp = append(resp, Send{appendEntries.leaderId,
					AppendEntriesRespEv{from: sm.id, term: sm.term, success: true}})
				if appendEntries.leaderCommit > sm.commitIndex {
					sm.commitIndex = min(appendEntries.leaderCommit, len(sm.log)-1)
				}
			} else {
				resp = append(resp, Send{appendEntries.leaderId,
					AppendEntriesRespEv{from: sm.id, term: sm.term, success: false}})
			}
		}
	case "Candidate", "Leader":
		if appendEntries.term < sm.term {
			resp = append(resp, Send{appendEntries.leaderId,
				AppendEntriesRespEv{from: sm.id, term: sm.term, success: false}})
		} else {
			sm.state = "Follower"
			sm.term = appendEntries.term
			resp = append(resp, StateStore{currentTerm: sm.term, votedFor: 0})
			resp = append(resp, Alarm{t: timeoutTime})
			if len(sm.log) > appendEntries.prevLogIndex &&
				sm.log[appendEntries.prevLogIndex].term == appendEntries.prevLogTerm {
				sm.log = append(sm.log[:appendEntries.prevLogIndex+1], appendEntries.entries...)
				index := appendEntries.prevLogIndex + 1
				i := 0
				for i < len(appendEntries.entries) {
					resp = append(resp,
						LogStore{index: index + i, term: sm.term, data: appendEntries.entries[i].data})
					i += 1
				}
				resp = append(resp, Send{appendEntries.leaderId,
					AppendEntriesRespEv{from: sm.id, term: sm.term, success: true}})
				if appendEntries.leaderCommit > sm.commitIndex {
					sm.commitIndex = min(appendEntries.leaderCommit, len(sm.log)-1)
				}
			} else {
				resp = append(resp, Send{appendEntries.leaderId,
					AppendEntriesRespEv{from: sm.id, term: sm.term, success: false}})
			}
		}
	}
	return resp
}

func (sm *StateMachine) appendEntriesResp(appendEntriesResp AppendEntriesRespEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower", "Candidate":
		if sm.term <= appendEntriesResp.term {
			println("ERROR: Inconsistent Input")
		}
	case "Leader":
		if sm.term == appendEntriesResp.term {
			if appendEntriesResp.success {
				newIndex := sm.nextIndex[appendEntriesResp.from]
				sm.matchIndex[appendEntriesResp.from] = newIndex
				sm.nextIndex[appendEntriesResp.from]++
				count := 1
				for _, peer := range sm.peers {
					if sm.matchIndex[peer] >= newIndex {
						count++
					}
				}
				if count > (len(sm.peers)+1)/2 {
					index := sm.commitIndex
					for index <= newIndex {
						resp = append(resp, Commit{index: index, data: sm.log[index].data, err: ""})
						index += 1
					}
					sm.commitIndex = newIndex
				}
			} else {
				sm.nextIndex[appendEntriesResp.from]--
				resp = append(resp, Send{appendEntriesResp.from, AppendEntriesReqEv{
					sm.term, sm.id, sm.nextIndex[appendEntriesResp.from],
					sm.log[sm.nextIndex[appendEntriesResp.from]].term, nil, sm.commitIndex}})
			}
		} else {
			sm.state = "Follower"
			sm.term = appendEntriesResp.term
			resp = append(resp, StateStore{currentTerm: sm.term, votedFor: 0})
			resp = append(resp, Alarm{t: timeoutTime})
		}
	}
	return resp
}

func (sm *StateMachine) append(appendEv AppendEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower", "Candidate":
		resp = append(resp, Commit{index: -1, data: appendEv.data, err: "ERR_NOT_LEADER"})
	case "Leader":
		sm.log = append(sm.log, logEntry{term: sm.term, data: appendEv.data})
		resp = append(resp, LogStore{index: len(sm.log) - 1, data: appendEv.data, term: sm.term})
		for _, peer := range sm.peers {
			resp = append(resp, Send{peer,
				AppendEntriesReqEv{sm.term, sm.id, sm.nextIndex[peer] - 1,
					sm.log[sm.nextIndex[peer]-1].term,
					sm.log[sm.nextIndex[peer]:], sm.commitIndex}})
		}
	}
	return resp
}

func getHeartBeatEvents(sm *StateMachine) []interface{} {
	var resp []interface{}
	for _, peer := range sm.peers {
		if sm.nextIndex[peer] == len(sm.log) {
			resp = append(resp, Send{peer, AppendEntriesReqEv{
				sm.term, sm.id, len(sm.log) - 1,
				sm.log[len(sm.log)-1].term, nil, sm.commitIndex}})
		} else {
			resp = append(resp, Send{peer, AppendEntriesReqEv{
				sm.term, sm.id, sm.nextIndex[peer] - 1,
				sm.log[sm.nextIndex[peer]-1].term, sm.log[sm.nextIndex[peer]:], sm.commitIndex}})
		}
	}
	return resp
}

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func (sm *StateMachine) ProcessEvent(ev interface{}) []interface{} {
	var outputActions []interface{}
	switch ev.(type) {
	case AppendEv:
		event := ev.(AppendEv)
		outputActions = sm.append(event)
	case TimeoutEv:
		outputActions = sm.timeout()
	case AppendEntriesReqEv:
		event := ev.(AppendEntriesReqEv)
		outputActions = sm.appendEntriesReq(event)
	case AppendEntriesRespEv:
		event := ev.(AppendEntriesRespEv)
		outputActions = sm.appendEntriesResp(event)
	case VoteReqEv:
		event := ev.(VoteReqEv)
		outputActions = sm.voteReq(event)
	case VoteRespEv:
		event := ev.(VoteRespEv)
		outputActions = sm.voteResp(event)
	default:
		println("Unrecognized")
	}
	if DEBUG {
		printOutputActions(outputActions)
	}
	return outputActions
}

func (sm *StateMachine) eventLoop() {
	for {
		select {
		case appendMsg := <-sm.clientCh:
			responses := sm.ProcessEvent(appendMsg)
			for _, response := range responses {
				sm.actionCh <- response
			}
		case peerMsg := <-sm.netCh:
			responses := sm.ProcessEvent(peerMsg)
			for _, response := range responses {
				sm.actionCh <- response
			}
		case <-sm.timeoutCh:
			responses := sm.ProcessEvent(TimeoutEv{})
			for _, response := range responses {
				sm.actionCh <- response
			}
		}
	}
}
