package assignment2
type AppendEv struct {
	data []byte
}

type TimeoutEv struct {
}

type Alarm struct {
	t int
}

type logEntry struct {
	term int
	data []byte
}

type AppendEntriesReqEv struct {
	term         int
	leaderId     int
	prevLogIndex int
	prevLogTerm  int
	entries      []logEntry
	leaderCommit int
}

type AppendEntriesRespEv struct {
	from    int
	term    int
	success bool
}

type VoteReqEv struct {
	term         int
	candidateId  int
	lastLogIndex int
	lastLogTerm  int
}

type VoteRespEv struct {
	from        int
	term        int
	voteGranted bool
}

type StateMachine struct {
	id          int // server id
	state       string
	peers       []int // other server ids
	term        int
	voteCount   int
	log         []logEntry
	votedFor    int
	commitIndex int
	nextIndex   map[int]int
	matchIndex  map[int]int
	actionCh chan interface{}
	netCh chan interface{}
	timeoutCh chan interface{}
	clientCh chan interface{}
}

type Commit struct {
	index int
	data  []byte
	err   string
}

type Send struct {
	peerId int
	event  interface{}
}

type StateStore struct {
	currentTerm int
	votedFor    int
}

type LogStore struct {
	index int
	term int
	data  []byte
}

