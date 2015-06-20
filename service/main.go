package main

import "io"
import "log"
import "net/http"
import "github.com/zetafunction/steam-monster-game/poller"
import "github.com/zetafunction/steam-monster-game/steam"

func main() {
	service := steam.NewApiService()

	log.Print("Performing initial data update...")
	dataUpdate, err := poller.StartNewGameScanner(service)
	if err != nil {
		log.Fatal("Unable to start game poller:", err)
	}

	dataRequests := make(chan chan []byte, 50)
	http.HandleFunc("/game-poller-data.json",
		func(w http.ResponseWriter, req *http.Request) {
			request := make(chan []byte)
			dataRequests <- request
			json := <-request
			io.WriteString(w, string(json))
		})
	log.Print("Starting HTTP server...")
	go func() {
		if err := http.ListenAndServe("127.0.0.1:2742", nil); err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()
	var json []byte
	for {
		select {
		case json = <-dataUpdate:
		case req := <-dataRequests:
			req <- json
		}
	}
}
