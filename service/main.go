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

	rangeFinder := poller.NewRangeFinder(service)
	rangeFinder.Start()
	newGameScanner := poller.NewNewGameScanner(service, rangeFinder)
	newGameScanner.Start()

	newGameScannerRequests := make(chan chan []byte, 50)
	http.HandleFunc("/new-game-scanner-data.json",
		func(w http.ResponseWriter, req *http.Request) {
			request := make(chan []byte)
			newGameScannerRequests <- request
			json := <-request
			io.WriteString(w, string(json))
		})
	log.Print("Starting HTTP server...")
	go func() {
		if err := http.ListenAndServe("127.0.0.1:2742", nil); err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()
	var newGameScannerData []byte
	for {
		select {
		case newGameScannerData = <-newGameScanner.GetUpdateChannel():
		case req := <-newGameScannerRequests:
			req <- newGameScannerData
		}
	}
}
