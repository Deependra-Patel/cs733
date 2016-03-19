package main

import "time"

type AppendEv struct {
	data []byte
}

type TimeoutEv struct {
}

type Alarm struct {
	t time.Duration
}

type logEntry struct {
	Term int
	Data []byte
}

type AppendEntriesReqEv struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []logEntry
	LeaderCommit int
}

type AppendEntriesRespEv struct {
	From    int
	Term    int
	Success bool
}

type VoteReqEv struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

type VoteRespEv struct {
	From        int
	Term        int
	VoteGranted bool
}

type StateMachine struct {
	id               int // server id
	leaderId         int //leader id
	state            string
	peers            []int // other server ids
	term             int
	voteCount        int
	ElectionTimeout  time.Duration
	HeartbeatTimeout time.Duration
	log              []logEntry
	votedFor         int
	commitIndex      int
	nextIndex        map[int]int
	matchIndex       map[int]int
	actionCh         chan interface{}
	netCh            chan interface{}
	timeoutCh        chan interface{}
	clientCh         chan interface{}
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
	term  int
	data  []byte
}

type Finish struct {
	//Dummy class to signal end of action
}
