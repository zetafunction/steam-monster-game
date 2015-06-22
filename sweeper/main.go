package main

import (
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/zetafunction/steam-monster-game/messages"
	"github.com/zetafunction/steam-monster-game/steam"
	"io/ioutil"
	"log"
)

var start = flag.Int("start", 1, "game to start scanning from")
var end = flag.Int("end", 49330, "game to end scanning at")

func main() {
	flag.Parse()

	api := steam.NewAPIService()

	type result struct {
		id     int
		result *steam.GameDataResult
	}
	c := make(chan result)
	requests := 0
	for i := *start; i < *end; i++ {
		go func(i int) {
			c <- result{i, <-api.GetGameData(i)}
		}(i)
		requests++
	}

	m := make(map[int]*steam.GameDataResult)
	for requests > 0 {
		if requests%1000 == 0 {
			log.Print("requests remaining: ", requests)
		}
		r := <-c
		m[r.id] = r.result
                requests--
	}

	for i := *start; i < *end; i++ {
		r := m[i]
		response := r.Response
		if r.Err != nil || response.GetGameData() == nil || response.GetStats() == nil {
			log.Print("failed to get data for ", i)
			continue
		}
		if response.GetGameData().GetStatus() != messages.EMiniGameStatus_k_EMiniGameStatus_Ended {
			log.Print("skipping non-ended game ", i)
			continue
		}
		serialized, err := proto.Marshal(response)
		if err != nil {
			log.Print("failed to serialize game ", i, ": ", err)
			continue
		}
		name := fmt.Sprintf("%05d.pb", i)
		if ioutil.WriteFile(name, serialized, 0644) != nil {
			log.Print("failed to write game ", i, " data to file: ", err)
			continue
		}
	}
}
