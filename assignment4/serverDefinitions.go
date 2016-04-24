package main
type serverConfig struct{
	id int
	raftNodeConfig Config
	serverAddressMap map[int]url
}

type url struct{
	host string
	port int
}