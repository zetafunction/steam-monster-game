package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/new-game-scanner",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "new-game-scanner.html")
		})
	http.HandleFunc("/new-game-scanner-dev",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "new-game-scanner-dev.html")
		})
	log.Fatal("ListenAndServe: ", http.ListenAndServe("127.0.0.1:2741", nil))
}
