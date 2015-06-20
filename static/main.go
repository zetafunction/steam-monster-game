package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/game-poller",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "game-poller.html")
		})
	http.HandleFunc("/game-poller-dev",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "game-poller-dev.html")
		})
	log.Fatal("ListenAndServe: ", http.ListenAndServe("127.0.0.1:2741", nil))
}
