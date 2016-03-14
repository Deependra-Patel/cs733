package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestAppend(t *testing.T) {
	data := []byte("somee data from client")
	sm := getSampleSM("Follower")
	errorMessage := "TestAppendFollower"
	sm.clientCh <- AppendEv{data: data}
	response := <-sm.actionCh
	expect(t, errorMessage, response, Commit{index: -1, data: data, err: "ERR_NOT_LEADER"})
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Candidate")
	errorMessage = "TestAppendCandidate"
	sm.clientCh <- AppendEv{data: data}
	response = <-sm.actionCh
	expect(t, errorMessage, response, Commit{index: -1, data: data, err: "ERR_NOT_LEADER"})
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Leader")
	initialSm := getSampleSM("Leader")
	errorMessage = "TestAppendLeader"
	sm.clientCh <- AppendEv{data: data}
	response = <-sm.actionCh
	expect(t, errorMessage, response, LogStore{index: 4, data: data, term: sm.term})
	for _, peer := range initialSm.peers {
		response = <-sm.actionCh
		expect(t, errorMessage, response, Send{peer,
			AppendEntriesReqEv{Term: initialSm.term, LeaderId: initialSm.id,
				PrevLogIndex: initialSm.nextIndex[peer] - 1,
				PrevLogTerm:  initialSm.log[initialSm.nextIndex[peer]-1].Term,
				Entries: append(getSampleLog()[initialSm.nextIndex[peer]:],
					logEntry{initialSm.term, data}),
				LeaderCommit: initialSm.commitIndex}})
	}
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)
}

func TestTimeout(t *testing.T) {
	sm := getSampleSM("Follower")
	initialSm := getSampleSM("Follower")
	errorMessage := "TestTimeoutFollower"
	sm.timeoutCh <- TimeoutEv{}
	expect(t, errorMessage, sm.state, "Candidate")
	expect(t, errorMessage, sm.voteCount, 1)
	expect(t, errorMessage, sm.votedFor, initialSm.id)
	expect(t, errorMessage, sm.term, initialSm.term+1)
	response := <-sm.actionCh
	expect(t, errorMessage, response, StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.id})
	response = <-sm.actionCh
	expect(t, errorMessage, response, Alarm{t: timeoutTime})
	for _, peer := range sm.peers {
		response = <-sm.actionCh
		expect(t, errorMessage, response, Send{peer, VoteReqEv{
			Term: initialSm.term + 1, CandidateId: initialSm.id, LastLogIndex: len(initialSm.log) - 1,
			LastLogTerm: initialSm.log[len(initialSm.log)-1].Term}})
	}
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	errorMessage = "TestTimeoutCandidate"
	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	sm.timeoutCh <- TimeoutEv{}
	expect(t, errorMessage, sm.state, "Candidate")
	expect(t, errorMessage, sm.voteCount, 1)
	expect(t, errorMessage, sm.votedFor, initialSm.id)
	expect(t, errorMessage, sm.term, initialSm.term+1)
	response = <-sm.actionCh
	expect(t, errorMessage, response, StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.id})
	response = <-sm.actionCh
	expect(t, errorMessage, response, Alarm{t: timeoutTime})
	for _, peer := range initialSm.peers {
		response = <-sm.actionCh
		expect(t, errorMessage, response, Send{peer, VoteReqEv{
			Term: initialSm.term + 1, CandidateId: initialSm.id, LastLogIndex: len(initialSm.log) - 1,
			LastLogTerm: initialSm.log[len(initialSm.log)-1].Term}})
	}
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	errorMessage = "TestTimeoutLeader"
	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	sm.timeoutCh <- TimeoutEv{}
	expect(t, errorMessage, sm.state, "Leader")
	expect(t, errorMessage, sm.term, initialSm.term)
	response = <-sm.actionCh
	expect(t, errorMessage, response, Alarm{t: timeoutTime})
	for _, peer := range sm.peers {
		response = <-sm.actionCh
		if len(initialSm.log) != initialSm.nextIndex[peer] {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				Term: initialSm.term, LeaderId: initialSm.id, PrevLogIndex: initialSm.nextIndex[peer] - 1,
				PrevLogTerm:  initialSm.log[initialSm.nextIndex[peer]-1].Term,
				Entries:      initialSm.log[initialSm.nextIndex[peer]:],
				LeaderCommit: initialSm.commitIndex}})
		} else {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				Term: initialSm.term, LeaderId: initialSm.id, PrevLogIndex: len(initialSm.log) - 1,
				PrevLogTerm: initialSm.log[len(initialSm.log)-1].Term, Entries: nil,
				LeaderCommit: initialSm.commitIndex}})
		}
	}
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)
}

func TestAppendEntriesReq(t *testing.T) {
	//with term lower than its
	sm := getSampleSM("Follower")
	initialSm := getSampleSM("Follower")
	sm.netCh <- AppendEntriesReqEv{LeaderId: initialSm.peers[0], Term: initialSm.term - 1}
	errorMessage := "TestAppendEntriesReqFollower"
	expectedActions := []interface{}{
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term, Success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	//check for higher term
	logEntries := []logEntry{logEntry{Term: initialSm.term + 2, Data: []byte("abd")},
		logEntry{Term: initialSm.term + 2, Data: []byte("bcd")}}
	sm.netCh <- AppendEntriesReqEv{Term: initialSm.term + 2, LeaderId: initialSm.peers[0],
		PrevLogIndex: 4, PrevLogTerm: 3, Entries: logEntries, LeaderCommit: 6}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 2, votedFor: 0},
		Alarm{t: timeoutTime},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term + 2, Success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm.netCh <- AppendEntriesReqEv{Term: initialSm.term + 2, LeaderId: initialSm.peers[0],
		PrevLogIndex: 3, PrevLogTerm: 2, Entries: logEntries, LeaderCommit: 4}
	expectedActions = []interface{}{
		Alarm{t: 10},
		LogStore{index: 4, term: initialSm.term + 2, data: []byte("abd")},
		LogStore{index: 5, term: initialSm.term + 2, data: []byte("bcd")},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term + 2, Success: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	sm.netCh <- AppendEntriesReqEv{LeaderId: initialSm.peers[0], Term: initialSm.term - 1}
	errorMessage = "TestAppendEntriesReqCandidate"
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term, Success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm.netCh <- AppendEntriesReqEv{Term: initialSm.term + 1, LeaderId: initialSm.peers[0],
		PrevLogIndex: 3, PrevLogTerm: 2, Entries: logEntries, LeaderCommit: 4}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Alarm{t: timeoutTime},
		LogStore{index: 4, term: initialSm.term + 1, data: []byte("abd")},
		LogStore{index: 5, term: initialSm.term + 1, data: []byte("bcd")},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term + 1, Success: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	expect(t, errorMessage, sm.state, "Follower")

	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	sm.netCh <- AppendEntriesReqEv{LeaderId: initialSm.peers[0], Term: initialSm.term - 1}
	errorMessage = "TestAppendEntriesReqLeader"
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term, Success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm.netCh <- AppendEntriesReqEv{Term: initialSm.term + 1, LeaderId: initialSm.peers[0],
		PrevLogIndex: 3, PrevLogTerm: 2, Entries: logEntries, LeaderCommit: 4}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Alarm{t: timeoutTime},
		LogStore{index: 4, term: initialSm.term + 1, data: []byte("abd")},
		LogStore{index: 5, term: initialSm.term + 1, data: []byte("bcd")},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{From: initialSm.id, Term: initialSm.term + 1, Success: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")
	checkEmptyChannel(t, errorMessage, sm)
}

func TestAppendEntriesResp(t *testing.T) {
	sm := getSampleSM("Follower")
	sm.netCh <- AppendEntriesRespEv{Term: sm.term - 1, From: sm.peers[0], Success: false}
	errorMessage := "TestAppendEntriesRespFollower"
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Candidate")
	sm.netCh <- AppendEntriesRespEv{Term: sm.term - 2, From: sm.peers[0], Success: false}
	errorMessage = "TestAppendEntriesRespCandidate"
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Leader")
	initialSm := getSampleSM("Leader")
	errorMessage = "TestAppendEntriesRespLeader"
	//append entry fail
	sm.netCh <- AppendEntriesRespEv{From: initialSm.peers[0], Term: initialSm.term, Success: false}
	expectedActions := []interface{}{
		Send{peerId: initialSm.peers[0], event: AppendEntriesReqEv{Term: initialSm.term, LeaderId: initialSm.id,
			PrevLogIndex: 1, PrevLogTerm: 1, Entries: initialSm.log[2:], LeaderCommit: initialSm.commitIndex}}}
	expectActions(t, errorMessage, sm, expectedActions)
	//commit case
	sm.netCh <- AppendEntriesRespEv{From: 4, Term: initialSm.term, Success: true}
	sm.netCh <- AppendEntriesRespEv{From: 5, Term: initialSm.term, Success: true}
	expectedActions = []interface{}{
		Commit{index: 2, data: initialSm.log[2].Data},
		Commit{index: 3, data: initialSm.log[3].Data}}
	expectActions(t, errorMessage, sm, expectedActions)
	expect(t, errorMessage, sm.commitIndex, 3)
	checkEmptyChannel(t, errorMessage, sm)
}

func TestVoteReq(t *testing.T) {
	sm := getSampleSM("Follower")
	initialSm := getSampleSM("Follower")
	errorMessage := "TestVoteReqFollower"
	//testing for term<sm.term
	sm.netCh <- VoteReqEv{Term: sm.term - 1, CandidateId: sm.peers[0]}
	expectedActions := []interface{}{
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	//term is greater but log not up to date
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 5, LastLogTerm: 1}
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 2, LastLogTerm: 2}
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 3, LastLogTerm: 2}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.peers[0]},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 1, LastLogTerm: 3}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 2, votedFor: initialSm.peers[0]},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 2,
			From: initialSm.id, VoteGranted: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	errorMessage = "TestVoteReqCandidate"
	//testing for term<=sm.term
	sm.netCh <- VoteReqEv{Term: sm.term, CandidateId: sm.peers[0]}
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	//term is greater but log not up to date
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 5, LastLogTerm: 1}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")

	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 2, LastLogTerm: 2}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")

	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 3, LastLogTerm: 2}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.peers[0]},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")

	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 1, LastLogTerm: 3}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.peers[0]},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")

	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	errorMessage = "TestVoteReqLeader"
	//testing for term<=sm.term
	sm.netCh <- VoteReqEv{Term: sm.term, CandidateId: sm.peers[0]}
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term,
			From: initialSm.id, VoteGranted: false}},
		Send{sm.peers[0], AppendEntriesReqEv{initialSm.term,
			initialSm.id, 1, 1, initialSm.log[2:], initialSm.commitIndex}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish

	//term is greater but log not up to date
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 5, LastLogTerm: 1}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")
	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 2, LastLogTerm: 2}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")
	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 3, LastLogTerm: 2}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.peers[0]},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")
	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	sm.netCh <- VoteReqEv{Term: sm.term + 1, CandidateId: sm.peers[0], LastLogIndex: 1, LastLogTerm: 3}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: initialSm.peers[0]},
		Send{peerId: initialSm.peers[0], event: VoteRespEv{Term: initialSm.term + 1,
			From: initialSm.id, VoteGranted: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	<- sm.actionCh //For dummy Finish
	expect(t, errorMessage, sm.state, "Follower")

	checkEmptyChannel(t, errorMessage, sm)
}

func TestVoteResp(t *testing.T) {
	sm := getSampleSM("Follower")
	sm.netCh <- VoteRespEv{Term: sm.term - 1, VoteGranted: true}
	errorMessage := "TestVoteRespFollower"
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Candidate")
	initialSm := getSampleSM("Candidate")
	errorMessage = "TestVoteRespCandidate"
	sm.netCh <- VoteRespEv{Term: sm.term, VoteGranted: true}
	expect(t, errorMessage, "Candidate", sm.state)
	<- sm.actionCh //For dummy Finish
	sm.netCh <- VoteRespEv{Term: sm.term, VoteGranted: false}
	expect(t, errorMessage, "Candidate", sm.state)
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)


	sm.netCh <- VoteRespEv{Term: sm.term, VoteGranted: true}
	expect(t, errorMessage, "Leader", sm.state)
	response := <-sm.actionCh
	expect(t, errorMessage, response, Alarm{t: timeoutTime})
	for _, peer := range sm.peers {
		response = <-sm.actionCh
		if len(initialSm.log) != initialSm.nextIndex[peer] {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				Term: initialSm.term, LeaderId: initialSm.id, PrevLogIndex: initialSm.nextIndex[peer] - 1,
				PrevLogTerm:  initialSm.log[initialSm.nextIndex[peer]-1].Term,
				Entries:      initialSm.log[initialSm.nextIndex[peer]:],
				LeaderCommit: initialSm.commitIndex}})
		} else {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				Term: initialSm.term, LeaderId: initialSm.id, PrevLogIndex: len(initialSm.log) - 1,
				PrevLogTerm: initialSm.log[len(initialSm.log)-1].Term, Entries: nil,
				LeaderCommit: initialSm.commitIndex}})
		}
	}
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Leader")
	sm.netCh <- VoteRespEv{Term: sm.term, VoteGranted: true}
	errorMessage = "TestVoteRespLeader"
	<- sm.actionCh //For dummy Finish
	checkEmptyChannel(t, errorMessage, sm)

}

func checkEmptyChannel(t *testing.T, errorMessage string, sm *StateMachine) {
	select {
	case _, ok := <-sm.actionCh:
		if ok {
			t.Error(errorMessage + " Extra event written to channel.\n")
		}
	default:
	}
}

func expectActions(t *testing.T, message string, sm *StateMachine, expectedActions []interface{}) {
	for _, expectedAction := range expectedActions {
		response := <-sm.actionCh
		expect(t, message, response, expectedAction)
	}
}

func expect(t *testing.T, message string, response interface{}, expected interface{}) {
	if !(reflect.DeepEqual(response, expected)) {
		t.Error(fmt.Sprintf("Message: %s Expected\n %+v\n found \n%+v", message, expected, response))
	}
}

func getSampleLog() []logEntry {
	sampleLog := []logEntry{logEntry{0, nil}, logEntry{1, []byte("firstLog")},
		logEntry{1, []byte("2ndLog")}, logEntry{2, []byte("3rdLog")}}
	return sampleLog
}

func getSampleSM(state string) *StateMachine {
	sm := &StateMachine{id: 2, term: 3, commitIndex: 1, state: state, peers: []int{1, 3, 4, 5},
		votedFor: 0, log: getSampleLog(), voteCount: 1,
		netCh: make(chan interface{}), timeoutCh: make(chan interface{}), actionCh: make(chan interface{}),
		clientCh: make(chan interface{}), matchIndex: map[int]int{1: 1, 3: 0, 4: 1, 5: 1},
		nextIndex: map[int]int{1: 2, 3: 1, 4: 2, 5: 2}}
	go func() {
		sm.eventLoop()
	}()
	return sm
}
