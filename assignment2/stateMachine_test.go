package assignment2

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
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Candidate")
	errorMessage = "TestAppendCandidate"
	sm.clientCh <- AppendEv{data: data}
	response = <-sm.actionCh
	expect(t, errorMessage, response, Commit{index: -1, data: data, err: "ERR_NOT_LEADER"})
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
			AppendEntriesReqEv{term: initialSm.term, leaderId: initialSm.id,
				prevLogIndex: initialSm.nextIndex[peer] - 1,
				prevLogTerm:  initialSm.log[initialSm.nextIndex[peer]-1].term,
				entries: append(getSampleLog()[initialSm.nextIndex[peer]:],
					logEntry{initialSm.term, data}),
				leaderCommit: initialSm.commitIndex}})
	}
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
			term: initialSm.term + 1, candidateId: initialSm.id, lastLogIndex: len(initialSm.log) - 1,
			lastLogTerm: initialSm.log[len(initialSm.log)-1].term}})
	}
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
			term: initialSm.term + 1, candidateId: initialSm.id, lastLogIndex: len(initialSm.log) - 1,
			lastLogTerm: initialSm.log[len(initialSm.log)-1].term}})
	}
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
				term: initialSm.term, leaderId: initialSm.id, prevLogIndex: initialSm.nextIndex[peer] - 1,
				prevLogTerm:  initialSm.log[initialSm.nextIndex[peer]-1].term,
				entries:      initialSm.log[initialSm.nextIndex[peer]:],
				leaderCommit: initialSm.commitIndex}})
		} else {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				term: initialSm.term, leaderId: initialSm.id, prevLogIndex: len(initialSm.log) - 1,
				prevLogTerm: initialSm.log[len(initialSm.log)-1].term, entries: nil,
				leaderCommit: initialSm.commitIndex}})
		}
	}
	checkEmptyChannel(t, errorMessage, sm)
}

func TestAppendEntriesReq(t *testing.T) {
	//with term lower than its
	sm := getSampleSM("Follower")
	initialSm := getSampleSM("Follower")
	sm.netCh <- AppendEntriesReqEv{leaderId: initialSm.peers[0], term: initialSm.term - 1}
	errorMessage := "TestAppendEntriesReqFollower"
	expectedActions := []interface{}{
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term, success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	//check for higher term
	logEntries := []logEntry{logEntry{term: initialSm.term + 2, data: []byte("abd")},
		logEntry{term: initialSm.term + 2, data: []byte("bcd")}}
	sm.netCh <- AppendEntriesReqEv{term: initialSm.term + 2, leaderId: initialSm.peers[0],
		prevLogIndex: 4, prevLogTerm: 3, entries: logEntries, leaderCommit: 6}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 2, votedFor: 0},
		Alarm{t: timeoutTime},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term + 2, success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)
	sm.netCh <- AppendEntriesReqEv{term: initialSm.term + 2, leaderId: initialSm.peers[0],
		prevLogIndex: 3, prevLogTerm: 2, entries: logEntries, leaderCommit: 4}
	expectedActions = []interface{}{
		Alarm{t: 10},
		LogStore{index: 4, term: initialSm.term + 2, data: []byte("abd")},
		LogStore{index: 5, term: initialSm.term + 2, data: []byte("bcd")},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term + 2, success: true}}}
	expectActions(t, errorMessage, sm, expectedActions)

	sm = getSampleSM("Candidate")
	initialSm = getSampleSM("Candidate")
	sm.netCh <- AppendEntriesReqEv{leaderId: initialSm.peers[0], term: initialSm.term - 1}
	errorMessage = "TestAppendEntriesReqCandidate"
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term, success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)

	sm.netCh <- AppendEntriesReqEv{term: initialSm.term + 1, leaderId: initialSm.peers[0],
		prevLogIndex: 3, prevLogTerm: 2, entries: logEntries, leaderCommit: 4}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Alarm{t: timeoutTime},
		LogStore{index: 4, term: initialSm.term + 1, data: []byte("abd")},
		LogStore{index: 5, term: initialSm.term + 1, data: []byte("bcd")},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term + 1, success: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	expect(t, errorMessage, sm.state, "Follower")

	sm = getSampleSM("Leader")
	initialSm = getSampleSM("Leader")
	sm.netCh <- AppendEntriesReqEv{leaderId: initialSm.peers[0], term: initialSm.term - 1}
	errorMessage = "TestAppendEntriesReqLeader"
	expectedActions = []interface{}{
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term, success: false}}}
	expectActions(t, errorMessage, sm, expectedActions)

	sm.netCh <- AppendEntriesReqEv{term: initialSm.term + 1, leaderId: initialSm.peers[0],
		prevLogIndex: 3, prevLogTerm: 2, entries: logEntries, leaderCommit: 4}
	expectedActions = []interface{}{
		StateStore{currentTerm: initialSm.term + 1, votedFor: 0},
		Alarm{t: timeoutTime},
		LogStore{index: 4, term: initialSm.term + 1, data: []byte("abd")},
		LogStore{index: 5, term: initialSm.term + 1, data: []byte("bcd")},
		Send{peerId: initialSm.peers[0],
			event: AppendEntriesRespEv{from: initialSm.id, term: initialSm.term + 1, success: true}}}
	expectActions(t, errorMessage, sm, expectedActions)
	expect(t, errorMessage, sm.state, "Follower")
	checkEmptyChannel(t, errorMessage, sm)
}

func TestAppendEntriesResp(t *testing.T) {
	sm := getSampleSM("Follower")
	sm.netCh <- AppendEntriesRespEv{term: sm.term - 1, from: sm.peers[0], success: false}
	errorMessage := "TestAppendEntriesRespFollower"
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Candidate")
	sm.netCh <- AppendEntriesRespEv{term: sm.term - 1, from: sm.peers[0], success: false}
	errorMessage = "TestAppendEntriesRespCandidate"
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Leader")
}

func TestVoteResp(t *testing.T) {
	sm := getSampleSM("Follower")
	sm.netCh <- VoteRespEv{term: sm.term - 1, voteGranted: true}
	errorMessage := "TestVoteRespFollower"
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Candidate")
	initialSm := getSampleSM("Candidate")
	errorMessage = "TestVoteRespCandidate"
	sm.netCh <- VoteRespEv{term: sm.term, voteGranted: true}
	expect(t, errorMessage, "Candidate", sm.state)
	sm.netCh <- VoteRespEv{term: sm.term, voteGranted: false}
	expect(t, errorMessage, "Candidate", sm.state)
	checkEmptyChannel(t, errorMessage, sm)
	sm.netCh <- VoteRespEv{term: sm.term, voteGranted: true}
	expect(t, errorMessage, "Leader", sm.state)
	response := <-sm.actionCh
	expect(t, errorMessage, response, Alarm{t: timeoutTime})
	for _, peer := range sm.peers {
		response = <-sm.actionCh
		if len(initialSm.log) != initialSm.nextIndex[peer] {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				term: initialSm.term, leaderId: initialSm.id, prevLogIndex: initialSm.nextIndex[peer] - 1,
				prevLogTerm:  initialSm.log[initialSm.nextIndex[peer]-1].term,
				entries:      initialSm.log[initialSm.nextIndex[peer]:],
				leaderCommit: initialSm.commitIndex}})
		} else {
			expect(t, errorMessage, response, Send{peer, AppendEntriesReqEv{
				term: initialSm.term, leaderId: initialSm.id, prevLogIndex: len(initialSm.log) - 1,
				prevLogTerm: initialSm.log[len(initialSm.log)-1].term, entries: nil,
				leaderCommit: initialSm.commitIndex}})
		}
	}
	checkEmptyChannel(t, errorMessage, sm)

	sm = getSampleSM("Leader")
	sm.netCh <- VoteRespEv{term: sm.term, voteGranted: true}
	errorMessage = "TestVoteRespLeader"
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
