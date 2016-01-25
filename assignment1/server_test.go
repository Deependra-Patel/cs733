//package main
package main
import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.T) {
	go serverMain()   // launch the server as a goroutine.
	time.Sleep(1 * time.Second)
}
func TestReadWrite(t *testing.T) {
	name := "hi.txt"
	contents := "bye"
	exptime := 300000
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}
	scanner := bufio.NewScanner(conn)

	// Test write+read
	fmt.Fprintf(conn, "write %v %v %v\r\n%v\r\n", name, len(contents), exptime, contents)
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	version, err := strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	fmt.Fprintf(conn, "read %v\r\n", name) // try a read now
	scanner.Scan()

	arr = strings.Split(scanner.Text(), " ")
	expect(t, arr[0], "CONTENTS")
	expect(t, arr[1], fmt.Sprintf("%v", version)) // expect only accepts strings, convert int version to string
	expect(t, arr[2], fmt.Sprintf("%v", len(contents)))
	scanner.Scan()
	expect(t, scanner.Text(), contents)
}

func TestCAS(t *testing.T) {
	name := "hi2.txt"
	contents := "first"
	contents2 := "second"
	exptime := 1000
	conn, err := net.Dial("tcp", "localhost:8080")
	scanner := bufio.NewScanner(conn)

	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}
	fmt.Fprintf(conn, "write %v %v %v\r\n%v\r\n", name, len(contents), exptime, contents)
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	version, err := strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	fmt.Fprintf(conn, "cas %v %v %v %v\r\n%v\r\n", name, version, len(contents2), exptime, contents2)
	scanner.Scan()
	resp = scanner.Text()
	arr = strings.Split(resp, " ")
	expect(t, arr[0], "OK")
	version, err = strconv.ParseInt(arr[1], 10, 64)
	if err != nil {
		t.Error("Non-numeric version found")
	}
	fmt.Fprintf(conn, "read %v\r\n", name)
	scanner.Scan()
	arr = strings.Split(scanner.Text(), " ")
	expect(t, arr[0], "CONTENTS")
	expect(t, arr[1], fmt.Sprintf("%v", version))
	expect(t, arr[2], fmt.Sprintf("%v", len(contents2)))
	scanner.Scan()
	expect(t, scanner.Text(), contents2)
}
func TestDeleteAndExpiry(t *testing.T) {
	name := "hi.txt"
	contents := "something"
	exptime := 1
	conn, err := net.Dial("tcp", "localhost:8080")
	scanner := bufio.NewScanner(conn)

	//Testing delete
	fmt.Fprintf(conn, "write %v %v\r\n%v\r\n", name, len(contents), contents)
	scanner.Scan()
	resp := scanner.Text()
	arr := strings.Split(resp, " ")
	expect(t, arr[0], "OK")
	_, err = strconv.ParseInt(arr[1], 10, 64)
	if err != nil {
		t.Error("Non-numeric version found")
	}
	fmt.Fprintf(conn, "delete %v\r\n", name)
	scanner.Scan()
	expect(t, scanner.Text(), "OK")
	fmt.Fprintf(conn, "read %v\r\n", name)
	scanner.Scan()
	expect(t, scanner.Text(), "ERR_FILE_NOT_FOUND")

	//Delete when file is expired should give ERR_FILE_NOT_FOUND
	fmt.Fprintf(conn, "write %v %v %v\r\n%v\r\n", name, len(contents), exptime, contents)
	scanner.Scan()
	resp = scanner.Text()
	arr = strings.Split(resp, " ")
	expect(t, arr[0], "OK")
	_, err = strconv.ParseInt(arr[1], 10, 64)
	if err != nil{
		t.Error("Non-numeric version found")
	}
	time.Sleep(time.Second*2) //waiting for file to expire
	fmt.Fprintf(conn, "delete %v\r\n", name)
	scanner.Scan()
	expect(t, scanner.Text(), "ERR_FILE_NOT_FOUND")
}
func TestFileContent(t *testing.T) {
	name := "hi.txt"
	contents3 := "xxcasabc write a 3\\adf cas delete hi.txt"
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}

	scanner := bufio.NewScanner(conn)
	//Testing with gibberish values in file content
	fmt.Fprintf(conn, "write %v %v\r\n%v\r\n", name, len(contents3), contents3)
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	version, err := strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	fmt.Fprintf(conn, "read %v\r\n", name) // try a read now
	scanner.Scan()
	arr = strings.Split(scanner.Text(), " ")
	expect(t, arr[0], "CONTENTS")
	expect(t, arr[1], fmt.Sprintf("%v", version))
	expect(t, arr[2], fmt.Sprintf("%v", len(contents3)))
	scanner.Scan()
	expect(t, scanner.Text(), contents3)
}

// Useful testing function
func expect(t *testing.T, a string, b string) {
	if a != b {
		t.Error(fmt.Sprintf("Expected %v, found %v", b, a)) // t.Error is visible when running `go test -verbose`
	}
}