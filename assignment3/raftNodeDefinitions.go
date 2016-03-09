package assignment3

import "github.com/Deependra-Patel/cs733/assignment3/assignment2/"

// Returns a Node object
func raftNew(config Config) Node

type RaftNode struct{
	Node
	eventChan chan interface{}
	timoutChan chan interface{}
}

type Node interface {
	// Client's message to Raft node
	Append([]byte)
	// A channel for client to listen on. What goes into Append must come out of here at some point.
	CommitChannel() <- chan CommitInfo
	// Last known committed index in the log.
	CommittedIndex() int //This could be -1 until the system stabilizes.
	// Returns the data at a log index, or an error.
	Get(index int) (error, []byte)
	// Node's id
	Id()
	// Id of leader. -1 if unknown
	LeaderId() int
	// Signal to shut down all goroutines, stop sockets, flush log and close it, cancel timers.
	Shutdown()
}

// data goes in via Append, comes out as CommitInfo from the node's CommitChannel
// Index is valid only if err == nil
type CommitInfo struct {
	Data []byte
	Index int // or int .. whatever you have in your code
	Err error
	// Err can be errred
}

// This is an example structure for Config .. change it to your convenience.
type Config struct {
	cluster []NetConfig // Information about all servers, including this.
	Id int // this node's id. One of the cluster's entries should match.
	LogDir string // Log file directory for this node
	ElectionTimeout int
	HeartbeatTimeout int
}

type NetConfig struct {
	Id int
	Host string
	Port int
}

