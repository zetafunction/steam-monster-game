package main

import (
	"flag"
	"log"
	"net/http"
)

var bindInterface = flag.String("bind", "127.0.0.1:2741", "interface to bind to")

func main() {
	flag.Parse()

	http.HandleFunc("/new-game-scanner",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "new-game-scanner.html")
		})
	http.HandleFunc("/new-game-scanner-dev",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "new-game-scanner-dev.html")
		})
	http.HandleFunc("/stat-crawler",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "stat-crawler.html")
		})
	http.HandleFunc("/stat-crawler-new",
		func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, "stat-crawler-new.html")
		})
	log.Fatal("ListenAndServe: ", http.ListenAndServe(*bindInterface, nil))
}
