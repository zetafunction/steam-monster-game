package main

import (
	"flag"
	"github.com/zetafunction/steam-monster-game/poller"
	"github.com/zetafunction/steam-monster-game/steam"
	"io"
	"log"
	"net/http"
)

var bindInterface = flag.String("bind", "127.0.0.1:2742", "interface to bind to")

func main() {
	flag.Parse()

	api := steam.NewAPIService()
	rangeFinder := poller.NewRangeFinder(api)
	newGameScanner := poller.NewNewGameScanner(api, rangeFinder)

	// The stat crawler uses its own instance of APIService to avoid blocking requests for
	// other, more critical, services.
	statCrawler := poller.NewStatCrawler(steam.NewAPIService(), rangeFinder)

	rangeFinder.Start()
	newGameScanner.Start()
	statCrawler.Start()

	newGameScannerRequests := make(chan chan []byte, 50)
	statCrawlerRequests := make(chan chan []byte, 50)
	http.HandleFunc("/new-game-scanner-data.json",
		func(w http.ResponseWriter, req *http.Request) {
			request := make(chan []byte)
			newGameScannerRequests <- request
			io.WriteString(w, string(<-request))
		})
	http.HandleFunc("/stat-crawler-data.json",
		func(w http.ResponseWriter, req *http.Request) {
			request := make(chan []byte)
			statCrawlerRequests <- request
			io.WriteString(w, string(<-request))
		})
	log.Print("Starting HTTP server...")
	go func() {
		if err := http.ListenAndServe(*bindInterface, nil); err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()
	var newGameScannerData []byte
	var statCrawlerData []byte
	for {
		select {
		case newGameScannerData = <-newGameScanner.GetUpdateChannel():
		case statCrawlerData = <-statCrawler.GetUpdateChannel():
		case req := <-newGameScannerRequests:
			req <- newGameScannerData
		case req := <-statCrawlerRequests:
			req <- statCrawlerData
		}
	}
}
