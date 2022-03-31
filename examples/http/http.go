package main

import (
	"fmt"
	"net/http"
	"time"
)

func hello() {
	http.ListenAndServe(":8081", nil)
	fmt.Println("hello")
}

func greet(w http.ResponseWriter, r *http.Request) {
	go hello()
	fmt.Fprintf(w, "Hello World! %s", time.Now())
}

func main() {
	http.HandleFunc("/", greet)
	http.ListenAndServe(":8080", nil)
}
