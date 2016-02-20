package assignment2
import (
	"testing"
)
func TestEventLoop (t *testing.T) {
	sampleLog := []logEntry{logEntry{0, []byte("ff")}, logEntry{1, []byte("dd")}, logEntry{1, []byte("jj")}}
	sm := StateMachine{id: 3, term: 3, commitIndex: 1, state: "Follower",
		votedFor: 0, log: sampleLog, netCh:make(chan interface{}), actionCh:make(chan interface{})}
	//sm.ProcessEvent(AppendEntriesReqEv{term : 10, prevLogIndex: 100, prevLogTerm: 3})
	go func(){
		sm.eventLoop()
	}()
	sm.netCh <- VoteReqEv{term: 2, candidateId: 1, lastLogIndex: 3, lastLogTerm: 1}
	<- sm.actionCh
}
