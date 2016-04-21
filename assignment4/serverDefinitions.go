package main
type serverConfig struct{
	id int
	raftNodeConfig Config
	host string
	port int
}