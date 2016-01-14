package main

import (
	"log"
	"net/http"
	"fmt"
	"sync"
	"time"
)
var mutexMap = make(map[string] *sync.Mutex)

func write(filename string, numBytes int, contentBytes []byte, expTime time.Time) string{
	return ""
}
func read(filename string) string{
	return ""
}
func cas(filename string, version int, contentBytes []byte, expTime time.Time) string{
	return ""
}

func delete(filename string) string{
	return ""
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Body)
	buffer := make([]byte, 10)
	r.Body.Read(buffer)
	fmt.Println(buffer)
}
func serverMain(){
	_ = mutexMap["deependra"]
	_ = write("first", 10, []byte("dfds"), time.Now())
	_ = read("first")
	_ = delete("first")
	_ = cas("first", 0, []byte(""), time.Now())
	http.HandleFunc("/", requestHandler)
	fmt.Println("Starting Server.")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func main() {
	serverMain()
}
