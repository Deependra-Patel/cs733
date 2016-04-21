package main

import (
	"bufio"
	"fmt"
	"github.com/Deependra-Patel/cs733/assignment4/fs"
	"net"
	"os"
	"strconv"
	"encoding/json"
)

var rNode RaftNode
var clientMap map[int]*net.TCPConn
var increasingClientId int

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
		msg, msgerr, fatalerr := fs.GetMsg(reader)
		if fatalerr != nil || msgerr != nil {
			reply(conn, &fs.Msg{Kind: 'M'})
			conn.Close()
			break
		}
		msg.ClientId = clientId
		data, err := json.Marshal(msg)
		check(err)
		rNode.Append(data)
		//if msgerr != nil {
		//	if (!reply(conn, &fs.Msg{Kind: 'M'})) {
		//		conn.Close()
		//		break
		//	}
		//}
	}
}

func commitHandler(){
	for {
		commitInfo := <-rNode.CommitChannel()
		if commitInfo.err == "" {
			binData := commitInfo.data
			var msg fs.Msg
			err := json.Unmarshal(binData, &msg)
			check(err)
			response := fs.ProcessMsg(&msg)
			conn := clientMap[msg.ClientId]
			if !reply(conn, response) {
				conn.Close()
				break
			}
		} else {
			fmt.Println("Error received in commit message")
		}
	}
}

func serverMain(sConfig serverConfig) {
	tcpaddr, err := net.ResolveTCPAddr("tcp", sConfig.host+":"+strconv.Itoa(sConfig.port))
	check(err)
	tcp_acceptor, err := net.ListenTCP("tcp", tcpaddr)
	check(err)
	increasingClientId = 1
	rNode = New(sConfig.raftNodeConfig)
	go commitHandler()
	for {
		tcp_conn, err := tcp_acceptor.AcceptTCP()
		check(err)
		go serve(tcp_conn, increasingClientId)
		increasingClientId += 1
	}
}

func main() {
}
