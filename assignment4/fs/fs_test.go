package fs

// This set of tests is meant to unit test the backend fs in isolation
// (without command-line parsing or network handling).
// fs.ProcessMsg is called with an input Msg, and the reply Msg is
// matched against an expected template.
// 
// The same tests are replicated in ../rpc_test, which is more of an
// end-to-end test

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)


func expect(t *testing.T, response *Msg, expected *Msg, errstr string) {
	ok := true
	if response.Kind != expected.Kind {
		ok = false
		errstr += fmt.Sprintf(" Got kind='%c', expected '%c'", response.Kind, expected.Kind)
	}
	if expected.Version > 0 && expected.Version != response.Version {
		ok = false
		errstr += " Version mismatch"
	}

	if response.Kind == 'C' {
		if expected.Contents != nil &&
			bytes.Compare(response.Contents, expected.Contents) != 0 {
			ok = false
		}
	}
	if !ok {
		t.Fatal("Expected " + errstr)
	}
}

func TestFS_BasicSequential(t *testing.T) {
	// Read non-existent file cs733
	m := ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'F'}, "file not found")

	// Delete non-existent file cs733
	m = ProcessMsg(&Msg{Kind: 'd', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'F'}, "file not found")

	// Write file cs733
	str := "Cloud fun"
	m = ProcessMsg(&Msg{Kind: 'w', Filename: "cs733", Contents: []byte(str)})
	expect(t, m, &Msg{Kind: 'O'}, "write success")

	// Expect to read it back
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str)}, "read my write")

	// CAS in new value
	version := m.Version
	str2 := "Cloud fun 2"
	// Cas new value
	m = ProcessMsg(&Msg{Kind: 'c', Filename: "cs733", Contents: []byte(str2), Version: version})
	expect(t, m, &Msg{Kind: 'O'}, "cas success")

	// Expect to read it back
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str2)}, "read my cas")

	// Expect Cas to fail with old version
	m = ProcessMsg(&Msg{Kind: 'c', Filename: "cs733", Contents: []byte(str), Version: version})
	expect(t, m, &Msg{Kind: 'V'}, "cas version mismatch")

	// Expect a failed cas to not have succeeded. Read should return str2.
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str2)}, "failed cas to not have succeeded")

	// delete
	m = ProcessMsg(&Msg{Kind: 'd', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'O'}, "delete success")

	// Expect to not find the file
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'F'}, "file not found")
}

func TestFS_BasicTimer(t *testing.T) {
	// Write file cs733, with expiry time of 2 seconds
	str := "Cloud fun"
	m := ProcessMsg(&Msg{Kind: 'w', Filename: "cs733", Contents: []byte(str), Exptime: 2})
	expect(t, m, &Msg{Kind: 'O'}, "write success")

	// Expect to read it back immediately.
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str)}, "read my cas")

	time.Sleep(3 * time.Second)
	// Expect to not find the file after expiry
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'F'}, "file not found")

	// Recreate the file with expiry time of 1 second
	m = ProcessMsg(&Msg{Kind: 'w', Filename: "cs733", Contents: []byte(str), Exptime: 1})
	expect(t, m, &Msg{Kind: 'O'}, "file recreated")

	// Overwrite the file with expiry time of 4. This should be the new time.
	m = ProcessMsg(&Msg{Kind: 'w', Filename: "cs733", Contents: []byte(str), Exptime: 3})
	expect(t, m, &Msg{Kind: 'O'}, "file overwriten with exptime=4")

	// The last expiry time was 3 seconds. We should expect the file to still be around 2 seconds later
	time.Sleep(2 * time.Second)
	// Expect the file to not have expired.
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'C', Contents: []byte(str)}, "file to not expire until 4 sec")

	time.Sleep(3 * time.Second)
	// 5 seconds since the last write. Expect the file to have expired
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "cs733"})
	expect(t, m, &Msg{Kind: 'F'}, "file not found after 4 sec")
}

func TestFS_ConcurrentWrites(t *testing.T) {

	// nclients write to the same file. At the end the file should be any one clients' last write
	// tests expiry as well.
	var sem sync.WaitGroup // Used as a semaphore to coordinate goroutines to begin concurrently
	sem.Add(1)
	nclients := 500
	niters := 10
	ch := make(chan *Msg, nclients*niters) // channel for all replies
	for i := 0; i < nclients; i++ {        // 100 clients
		go func(i int) {
			sem.Wait()
			for j := 0; j < niters; j++ {
				str := fmt.Sprintf("cl %d %d", i, j)
				m := ProcessMsg(&Msg{Kind: 'w', Filename: "concWrite", Contents: []byte(str)})
				ch <- m
			}
		}(i)
	}
	time.Sleep(100 * time.Millisecond) // give goroutines a chance
	sem.Done()                         // Go!

	// There should be no errors
	for i := 0; i < nclients*niters; i++ {
		m := <-ch
		if m.Kind != 'O' {
			t.Fatalf("Concurrent write failed with kind=%c", m.Kind)
		}
	}
	m := ProcessMsg(&Msg{Kind: 'r', Filename: "concWrite"})
	// Ensure the contents are of the form "cl <i> 9"
	// The last write of any client ends with " 9"
	if m.Kind != 'C' || !strings.HasSuffix(string(m.Contents), " 9") {
		t.Fatalf("Expected to be able to read after 1000 writes")
	}
}

func TestFS_ConcurrentCas(t *testing.T) {

	// nclients write to the same file. At the end the file should be any one clients' last write
	// tests expiry as well.
	var sem sync.WaitGroup // Used as a semaphore to coordinate goroutines to *begin* concurrently
	sem.Add(1)
	nclients := 100
	niters := 10
	m := ProcessMsg(&Msg{Kind: 'w', Filename: "concCas"})
	ver := m.Version
	if m.Kind != 'O' || ver == 0 {
		t.Fatalf("Expected write to succeed and return version")
	}

	var wg sync.WaitGroup
	wg.Add(nclients)
	for i := 0; i < nclients; i++ {
		go func(i int, ver int) {
			sem.Wait()
			defer wg.Done()
			for j := 0; j < niters; j++ {
				str := fmt.Sprintf("cl %d %d", i, j)
				for {
					casMsg := &Msg{Kind: 'c', Filename: "concWrite", Contents: []byte(str), Version: ver}
					m := ProcessMsg(casMsg)
					if m.Kind == 'O' {
						break
					} else if m.Kind != 'V' {
						t.Fatal("Unexpected error in cas")
					}

					ver = m.Version // retry with latest version
				}
			}
		}(i, ver)
	}

	time.Sleep(100 * time.Millisecond) // give goroutines a chance
	sem.Done()                         // Start goroutines
	wg.Wait()                          // Wait for them to finish
	m = ProcessMsg(&Msg{Kind: 'r', Filename: "concWrite"})
	if m.Kind != 'C' || !strings.HasSuffix(string(m.Contents), " 9") {
		t.Fatalf("Expected to be able to read after 1000 writes")
	}
}
