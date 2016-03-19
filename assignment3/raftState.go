package main

import (
	"fmt"
	"math/rand"
	"time"
)

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
		if sm.term <= voteReq.Term &&
			(sm.votedFor == 0 || sm.votedFor == voteReq.CandidateId) &&
			((voteReq.LastLogTerm > sm.log[len(sm.log)-1].Term) ||
				((voteReq.LastLogTerm == sm.log[len(sm.log)-1].Term) && (voteReq.LastLogIndex >= (len(sm.log) - 1)))) {
			sm.term = voteReq.Term
			sm.votedFor = voteReq.CandidateId
			resp = append(resp, StateStore{sm.term, sm.votedFor})
			resp = append(resp, Send{voteReq.CandidateId,
				VoteRespEv{Term: sm.term, VoteGranted: true, From: sm.id}})
		} else {
			resp = append(resp, Send{voteReq.CandidateId,
				VoteRespEv{Term: sm.term, VoteGranted: false, From: sm.id}})
		}
	case "Candidate":
		if sm.term < voteReq.Term {
			sm.state = "Follower"
			sm.term = voteReq.Term
			if voteReq.LastLogTerm > sm.log[len(sm.log)-1].Term ||
				((voteReq.LastLogTerm == sm.log[len(sm.log)-1].Term) && voteReq.LastLogIndex >= (len(sm.log)-1)) {
				sm.votedFor = voteReq.CandidateId
				resp = append(resp, StateStore{sm.term, sm.votedFor})
				resp = append(resp, Send{voteReq.CandidateId,
					VoteRespEv{Term: sm.term, VoteGranted: true, From: sm.id}})
			} else {
				sm.votedFor = 0
				resp = append(resp, StateStore{sm.term, 0})
				resp = append(resp, Send{voteReq.CandidateId,
					VoteRespEv{Term: sm.term, VoteGranted: false, From: sm.id}})
			}
		} else {
			resp = append(resp, Send{voteReq.CandidateId,
				VoteRespEv{Term: sm.term, VoteGranted: false, From: sm.id}})
		}
	case "Leader":
		if sm.term >= voteReq.Term {
			resp = append(resp, Send{voteReq.CandidateId,
				VoteRespEv{Term: sm.term, VoteGranted: false, From: sm.id}})
			resp = append(resp, Send{voteReq.CandidateId, AppendEntriesReqEv{sm.term,
				sm.id, sm.nextIndex[voteReq.CandidateId] - 1, sm.log[sm.nextIndex[voteReq.CandidateId]-1].Term,
				sm.log[sm.nextIndex[voteReq.CandidateId]:], sm.commitIndex}})
		} else {
			sm.state = "Follower"
			sm.term = voteReq.Term
			if voteReq.LastLogTerm > sm.log[len(sm.log)-1].Term ||
				((voteReq.LastLogTerm == sm.log[len(sm.log)-1].Term) && voteReq.LastLogIndex >= (len(sm.log)-1)) {
				sm.votedFor = voteReq.CandidateId
				resp = append(resp, StateStore{sm.term, sm.votedFor})
				resp = append(resp, Send{voteReq.CandidateId, VoteRespEv{Term: sm.term,
					VoteGranted: true, From: sm.id}})
			} else { //reject
				sm.votedFor = 0
				resp = append(resp, StateStore{sm.term, 0})
				resp = append(resp, Send{voteReq.CandidateId, VoteRespEv{Term: sm.term,
					VoteGranted: false, From: sm.id}})
			}
		}
	}
	return resp
}

func (sm *StateMachine) voteResp(voteResp VoteRespEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
		if sm.term <= voteResp.Term {
			if voteResp.VoteGranted {
				println("ERROR: Inconsistent Input")
			} else if sm.term < voteResp.Term{
				sm.term = voteResp.Term
				resp = append(resp, StateStore{sm.term, sm.votedFor})
			}
		}
	case "Candidate":
		if voteResp.Term == sm.term && voteResp.VoteGranted {
			sm.voteCount++
			if sm.voteCount >= (len(sm.peers)+3)/2 {
				sm.state = "Leader"
				fmt.Println("LEADER ELECTED", sm.id)
				for _, peer := range sm.peers {
					sm.nextIndex[peer] = len(sm.log)
					sm.matchIndex[peer] = 0
				}
				resp = append(resp, Alarm{t: sm.HeartbeatTimeout})
				resp = append(resp, getHeartBeatEvents(sm)...)
			}
		}
	case "Leader":
		if sm.term < voteResp.Term {
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
		resp = append(resp, Alarm{sm.ElectionTimeout})
		for _, peer := range sm.peers {
			resp = append(resp, Send{peer, VoteReqEv{Term: sm.term, CandidateId: sm.id,
				LastLogIndex: len(sm.log) - 1, LastLogTerm: sm.log[len(sm.log)-1].Term}})
		}
	case "Candidate":
		sm.term++
		sm.voteCount = 1
		sm.votedFor = sm.id
		resp = append(resp, StateStore{sm.term, sm.votedFor})
		rand.Seed(time.Now().UTC().UnixNano())
		resp = append(resp, Alarm{sm.ElectionTimeout+time.Duration(rand.Int63n(sm.ElectionTimeout.Nanoseconds()))})
		for _, peer := range sm.peers {
			resp = append(resp, Send{peer, VoteReqEv{Term: sm.term, CandidateId: sm.id,
				LastLogIndex: len(sm.log) - 1, LastLogTerm: sm.log[len(sm.log)-1].Term}})
		}
	case "Leader":
		resp = append(resp, Alarm{sm.HeartbeatTimeout})
		resp = append(resp, getHeartBeatEvents(sm)...)
	}
	return resp
}

func (sm *StateMachine) appendEntriesReq(appendEntries AppendEntriesReqEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower":
		if appendEntries.Term < sm.term {
			resp = append(resp, Send{appendEntries.LeaderId,
				AppendEntriesRespEv{From: sm.id, Term: sm.term, Success: false}})
		} else {
			if appendEntries.Term > sm.term {
				sm.term = appendEntries.Term
				sm.votedFor = 0
				resp = append(resp, StateStore{currentTerm: sm.term, votedFor: 0})
			}
			resp = append(resp, Alarm{t: sm.ElectionTimeout})
			if len(sm.log) > appendEntries.PrevLogIndex &&
				sm.log[appendEntries.PrevLogIndex].Term == appendEntries.PrevLogTerm {
				sm.log = append(sm.log[:appendEntries.PrevLogIndex +1], appendEntries.Entries...)
				index := appendEntries.PrevLogIndex + 1
				i := 0
				for i < len(appendEntries.Entries) {
					resp = append(resp, LogStore{index: index + i, term: sm.term,
						data: appendEntries.Entries[i].Data})
					i += 1
				}
				resp = append(resp, Send{appendEntries.LeaderId,
					AppendEntriesRespEv{From: sm.id, Term: sm.term, Success: true}})
				if appendEntries.LeaderCommit > sm.commitIndex {
					index := sm.commitIndex + 1
					sm.commitIndex = min(appendEntries.LeaderCommit, len(sm.log)-1)
					for index <= sm.commitIndex {
						resp = append(resp, Commit{index: index, data: sm.log[index].Data, err: ""})
						index++
					}
				}
			} else {
				resp = append(resp, Send{appendEntries.LeaderId,
					AppendEntriesRespEv{From: sm.id, Term: sm.term, Success: false}})
			}
		}
	case "Candidate", "Leader":
		if appendEntries.Term < sm.term {
			resp = append(resp, Send{appendEntries.LeaderId,
				AppendEntriesRespEv{From: sm.id, Term: sm.term, Success: false}})
		} else {
			sm.state = "Follower"
			sm.term = appendEntries.Term
			sm.votedFor = 0
			resp = append(resp, StateStore{currentTerm: sm.term, votedFor: 0})
			resp = append(resp, Alarm{t: sm.ElectionTimeout})
			if len(sm.log) > appendEntries.PrevLogIndex &&
				sm.log[appendEntries.PrevLogIndex].Term == appendEntries.PrevLogTerm {
				sm.log = append(sm.log[:appendEntries.PrevLogIndex +1], appendEntries.Entries...)
				index := appendEntries.PrevLogIndex + 1
				i := 0
				for i < len(appendEntries.Entries) {
					resp = append(resp,
						LogStore{index: index + i, term: sm.term, data: appendEntries.Entries[i].Data})
					i += 1
				}
				resp = append(resp, Send{appendEntries.LeaderId,
					AppendEntriesRespEv{From: sm.id, Term: sm.term, Success: true}})
				if appendEntries.LeaderCommit > sm.commitIndex {
					index := sm.commitIndex + 1
					sm.commitIndex = min(appendEntries.LeaderCommit, len(sm.log)-1)
					for index <= sm.commitIndex {
						resp = append(resp, Commit{index: index, data: sm.log[index].Data, err: ""})
						index++
					}
				}
			} else {
				resp = append(resp, Send{appendEntries.LeaderId,
					AppendEntriesRespEv{From: sm.id, Term: sm.term, Success: false}})
			}
		}
	}
	return resp
}

func (sm *StateMachine) appendEntriesResp(appendEntriesResp AppendEntriesRespEv) []interface{} {
	var resp []interface{}
	switch sm.state {
	case "Follower", "Candidate":
		if sm.term <= appendEntriesResp.Term {
			println("ERROR: Inconsistent Input")
		}
	case "Leader":
		if sm.term == appendEntriesResp.Term {
			if appendEntriesResp.Success {
				newIndex := min(len(sm.log)-1, sm.nextIndex[appendEntriesResp.From])
				sm.matchIndex[appendEntriesResp.From] = newIndex
				sm.nextIndex[appendEntriesResp.From] = len(sm.log)
				count := 1
				for _, peer := range sm.peers {
					if sm.matchIndex[peer] >= newIndex {
						count++
					}
				}
				if count > (len(sm.peers) + 1) / 2 {
					index := sm.commitIndex + 1
					for index <= newIndex {
						resp = append(resp, Commit{index: index, data: sm.log[index].Data, err: ""})
						index += 1
					}
					sm.commitIndex = newIndex
				}
			} else {
				sm.nextIndex[appendEntriesResp.From]--
				resp = append(resp, Send{appendEntriesResp.From, AppendEntriesReqEv{
					sm.term, sm.id, sm.nextIndex[appendEntriesResp.From],
					sm.log[sm.nextIndex[appendEntriesResp.From]].Term,
					sm.log[sm.nextIndex[appendEntriesResp.From]+1:], sm.commitIndex}})
			}
		} else {
			sm.state = "Follower"
			sm.term = appendEntriesResp.Term
			sm.votedFor = 0
			resp = append(resp, StateStore{currentTerm: sm.term, votedFor: 0})
			resp = append(resp, Alarm{t: sm.ElectionTimeout})
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
		sm.log = append(sm.log, logEntry{Term: sm.term, Data: appendEv.data})
		resp = append(resp, LogStore{index: len(sm.log) - 1, data: appendEv.data, term: sm.term})
		for _, peer := range sm.peers {
			resp = append(resp, Send{peer,
				AppendEntriesReqEv{sm.term, sm.id, sm.nextIndex[peer] - 1,
					sm.log[sm.nextIndex[peer]-1].Term,
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
				sm.log[len(sm.log)-1].Term, nil, sm.commitIndex}})
		} else {
			resp = append(resp, Send{peer, AppendEntriesReqEv{
				sm.term, sm.id, sm.nextIndex[peer] - 1,
				sm.log[sm.nextIndex[peer]-1].Term, sm.log[sm.nextIndex[peer]:], sm.commitIndex}})
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
			sm.actionCh <- Finish{}
		case peerMsg := <-sm.netCh:
			responses := sm.ProcessEvent(peerMsg)
			for _, response := range responses {
				sm.actionCh <- response
			}
			sm.actionCh <- Finish{}
		case <-sm.timeoutCh:
			responses := sm.ProcessEvent(TimeoutEv{})
			for _, response := range responses {
				sm.actionCh <- response
			}
			sm.actionCh <- Finish{}
		}
	}
}
