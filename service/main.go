package main

import (
	"github.com/zetafunction/steam-monster-game/poller"
	"github.com/zetafunction/steam-monster-game/steam"
	"io"
	"log"
	"net/http"
)

func main() {
	service := steam.NewAPIService()

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
