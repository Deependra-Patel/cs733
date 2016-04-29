package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/Deependra-Patel/cs733/assignment4/fs"
	"net"
	"os"
	"strconv"
	"time"
	//logger "log"
	"sync"
)

var rNode RaftNode
var clientMap map[int]*net.TCPConn
var increasingClientId int
var serverUrlMap map[int]string
var lock = &sync.Mutex{}

var crlf = []byte{'\r', '\n'}

func check(obj interface{}) {
	if obj != nil {
		fmt.Println(obj)
		os.Exit(1)
	}
}

func reply(conn *net.TCPConn, msg *fs.Msg) bool {
	var err error
	write := func(data []byte) {
		if err != nil {
			return
		}
		_, err = conn.Write(data)
	}
	var resp string
	switch msg.Kind {
	case 'C': // read response
		resp = fmt.Sprintf("CONTENTS %d %d %d", msg.Version, msg.Numbytes, msg.Exptime)
	case 'O':
		resp = "OK "
		if msg.Version > 0 {
			resp += strconv.Itoa(msg.Version)
		}
	case 'F':
		resp = "ERR_FILE_NOT_FOUND"
	case 'V':
		resp = "ERR_VERSION " + strconv.Itoa(msg.Version)
	case 'M':
		resp = "ERR_CMD_ERR"
	case 'I':
		resp = "ERR_INTERNAL"
	case 'R':
		resp = "ERR_REDIRECT " + string(msg.Contents)
	default:
		fmt.Printf("Unknown response kind '%c'", msg.Kind)
		return false
	}
	resp += "\r\n"
	write([]byte(resp))
	if msg.Kind == 'C' {
		write(msg.Contents)
		write(crlf)
	}
	return err == nil
}

func serve(conn *net.TCPConn, clientId int) {
	reader := bufio.NewReader(conn)
	for {
		if rNode.LeaderId() != rNode.Id() {
			leaderId := rNode.LeaderId()
			//logger.Println("Redirecting client to leader: ", leaderId)
			if leaderId != -1 {
				//logger.Println("Leader url: ", serverUrlMap[leaderId])
				reply(conn, &fs.Msg{Kind: 'R', Contents: []byte(serverUrlMap[leaderId])})
			} else {
				reply(conn, &fs.Msg{Kind: 'R', Contents: []byte("-1")})
			}
			conn.Close()
			break
		}
		msg, msgerr, fatalerr := fs.GetMsg(reader)
		if fatalerr != nil || msgerr != nil {
			reply(conn, &fs.Msg{Kind: 'M'})
			conn.Close()
			break
		}
		msg.ClientId = clientId
		data, err := json.Marshal(msg)
		check(err)
		//logger.Println("Received message: ", msg.Kind, string(msg.Contents))
		lock.Lock()
		clientMap[clientId] = conn
		lock.Unlock()
		rNode.Append(data)
	}
}

func commitHandler() {
	//fmt.Println("Starting new thread for handling commits")
	for {
		commitInfo := <-rNode.CommitChannel()
		var response *fs.Msg
		var msg fs.Msg
		binData := commitInfo.data
		err := json.Unmarshal(binData, &msg)
		check(err)
		lock.Lock()
		conn := clientMap[msg.ClientId]
		lock.Unlock()

		if commitInfo.err == "" {
			response = fs.ProcessMsg(&msg)
			if conn != nil {
				//logger.Println("Message: ", string(msg.Kind), msg)
				if !reply(conn, response) {
					conn.Close()
					break
				}
			}
		} else {
			fmt.Println("Error received in commit message: ", commitInfo.err)
		}
	}
}

func serverMain(sConfig serverConfig) {
	clientMap = make(map[int]*net.TCPConn)
	serverUrlMap = make(map[int]string)
	for id, url := range sConfig.serverAddressMap {
		serverUrlMap[id] = url.host + ":" + strconv.Itoa(url.port)
	}
	tcpaddr, err := net.ResolveTCPAddr("tcp", serverUrlMap[sConfig.id])
	check(err)
	tcp_acceptor, err := net.ListenTCP("tcp", tcpaddr)
	check(err)
	increasingClientId = 1
	rNode = New(sConfig.raftNodeConfig)
	time.Sleep(1 * time.Second)
	go rNode.processEvents()
	go commitHandler()

	//fmt.Println("Started server: ", serverUrlMap[sConfig.id])
	for {
		tcp_conn, err := tcp_acceptor.AcceptTCP()
		check(err)
		go serve(tcp_conn, increasingClientId)
		increasingClientId += 1
	}
}

func main() {
	nodeId, err := strconv.Atoi(os.Args[1])
	check(err)
	sConfigs := getServerConfigs(5, 8000)
	serverMain(sConfigs[nodeId-1])
}

func getRaftConfigs(n int, port int) []Config {
	netConfigs := make([]NetConfig, n)
	for i := 0; i < n; i++ {
		netConfigs[i] = NetConfig{Id: i + 1, Host: "localhost", Port: port + i}
	}

	config := Config{
		cluster:          netConfigs,
		Id:               1,
		LogDir:           "mylog",
		ElectionTimeout:  time.Millisecond * time.Duration(2000),
		HeartbeatTimeout: time.Millisecond * time.Duration(500),
	}

	configs := make([]Config, 0)
	for i := 0; i < n; i++ {
		temp := config
		temp.Id = i + 1
		temp.LogDir = temp.LogDir + strconv.Itoa(i+1)
		configs = append(configs, temp)
	}
	return configs
}

func getServerConfigs(n int, port int) []serverConfig {
	raftConfigs := getRaftConfigs(n, port+100)
	serverConfigs := make([]serverConfig, n)
	serverAddressMap := make(map[int]url)
	for i := 0; i < n; i++ {
		serverAddressMap[i+1] = url{host: "localhost", port: port + i}
	}
	for i := 0; i < n; i++ {
		serverConfigs[i].id = i + 1
		serverConfigs[i].serverAddressMap = serverAddressMap
		serverConfigs[i].raftNodeConfig = raftConfigs[i]
	}
	return serverConfigs
}
