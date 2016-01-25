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
	contents1 := "abc \r\n delete hi.txt \\ cas read"
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}

	scanner := bufio.NewScanner(conn)
	//Testing with gibberish values in file content
	fmt.Fprintf(conn, "write %v %v\r\n%v\r\n", name, len(contents1), contents1)
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
	expect(t, arr[2], fmt.Sprintf("%v", len(contents1)))
	scanner.Scan()
	expect(t, scanner.Text(), "abc ")
	scanner.Scan()
	expect(t, scanner.Text(), " delete hi.txt \\ cas read")
}

func TestWriteConcurrency(t *testing.T) {
	name := "hi.txt"
	n := 100
	c := make(chan int64)
	for i:=0; i<n; i++{
		go writeFunc(name, strconv.Itoa(i), t, c)
	}
	for i:=0; i<n; i++{
		<- c
	}

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}
	scanner := bufio.NewScanner(conn)
	fmt.Fprintf(conn, "read %v\r\n", name) // try a read now
	scanner.Scan()
	arr := strings.Split(scanner.Text(), " ")
	expect(t, arr[0], "CONTENTS")
	_, err = strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	scanner.Scan()
	content, err := strconv.Atoi(scanner.Text())
	if err!=nil{
		t.Error("Non integral value in content")
	}
	if (content < 0 || content >= n){
		t.Error("Content not found as given.")
	}
}

func TestCASConcurrency(t *testing.T) {
	name := "hi.txt"
	contents := "initial content"
	newcontent := "replace with this"
	n := 100
	c := make(chan string)

	conn, err := net.Dial("tcp", "localhost:8080")
	scanner := bufio.NewScanner(conn)
	fmt.Fprintf(conn, "write %v %v\r\n%v\r\n", name, len(contents), contents)
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	version, err := strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}

	for i:=0; i<n; i++{
		go casFunc(name, version, t, c, newcontent)
	}
	countOK := 0
	countERR := 0
	for i:=0; i<n; i++{
		response := <- c
		if (response == "OK"){
			countOK++
		} else {
			countERR++
		}
	}
	if (countOK != 1 || (countOK + countERR)!=n){
		t.Error("Error in cas with concurrency.")
	}
	//now reading value after cas
	fmt.Fprintf(conn, "read %v\r\n", name) // try a read now
	scanner.Scan()
	arr = strings.Split(scanner.Text(), " ")
	expect(t, arr[0], "CONTENTS")
	_, err = strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	expect(t, arr[2], fmt.Sprintf("%v", len(newcontent)))
	scanner.Scan()
	expect(t, scanner.Text(), newcontent)
}

func casFunc(name string, version int64, t *testing.T, c chan string, contents string){
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}
	scanner := bufio.NewScanner(conn)
	fmt.Fprintf(conn, "cas %v %v %v\r\n%v\r\n", name, version, len(contents), contents)
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ")
	if len(arr) == 1 {
		expect(t, arr[0], "ERR_VERSION")
		c <- "ERR_VERSION";
	} else if len(arr) == 2 {
		expect(t, arr[0], "OK")
		_, err := strconv.ParseInt(arr[1], 10, 64) // parse version as number
		if err != nil {
			t.Error("Non-numeric version found")
		}
		c <- "OK"
	}
}

func writeFunc(name string, contents string, t *testing.T, c chan int64){
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}
	scanner := bufio.NewScanner(conn)
	fmt.Fprintf(conn, "write %v %v\r\n%v\r\n", name, len(contents), contents)
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	version, err := strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	c <- version;
}
// Useful testing function
func expect(t *testing.T, a string, b string) {
	if a != b {
		t.Error(fmt.Sprintf("Expected %v, found %v", b, a)) // t.Error is visible when running `go test -verbose`
	}
}