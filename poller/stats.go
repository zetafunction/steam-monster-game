package poller

import (
	"encoding/json"
	"github.com/zetafunction/steam-monster-game/messages"
	"github.com/zetafunction/steam-monster-game/steam"
	"log"
	"time"
)

type StatCrawler struct {
	api  *steam.APIService
	quit chan struct{}

	nonEndedUpdate chan int
	nonEnded       int

	invalidUpdate chan int
	invalid       int

	update chan []byte
}

func NewStatCrawler(api *steam.APIService, finder *RangeFinder) *StatCrawler {
	invalidUpdate := finder.SubscribeInvalid()
	nonEndedUpdate := finder.SubscribeNonEnded()
	return &StatCrawler{
		api,
		make(chan struct{}),
		nonEndedUpdate,
		<-nonEndedUpdate,
		invalidUpdate,
		<-invalidUpdate,
		make(chan []byte),
	}
}

func (c *StatCrawler) Start() {
	go func() {
		t := time.After(time.Second * 30)
		for {
			select {
			case <-t:
				func() {
					json, err := c.updateData()
					if err != nil {
						log.Print("stat crawler: update failed: ", err)
						return
					}
					c.update <- json
				}()
				t = time.After(time.Second * 30)
			case c.invalid = <-c.invalidUpdate:
			case c.nonEnded = <-c.nonEndedUpdate:
			}
		}
	}()
}

func (c *StatCrawler) Stop() {
	// TODO: This kind of gives the impression that things can be cleanly shut down.
	// Maybe just remove all the Stop() method receivers, since that really isn't the case.
	close(c.quit)
}

func (c *StatCrawler) GetUpdateChannel() <-chan []byte {
	return c.update
}

func (c *StatCrawler) updateData() ([]byte, error) {
	log.Print("stat crawler: updating from ", c.nonEnded, " to ", c.invalid)
	type update struct {
		id     int
		result *steam.GameDataResult
	}
	updates := make(chan update)
	requests := 0
	for i := c.nonEnded; i < c.invalid; i++ {
		go func(i int) {
			updates <- update{i, <-c.api.GetGameData(i)}
		}(i)
		requests++
	}
	m := make(map[int]*steam.GameDataResult)
	for requests > 0 {
		update := <-updates
		m[update.id] = update.result
		requests--
	}
	type statEntry struct {
		ID            int
		Level         uint32
		GameStart     uint32
		LevelStart    uint32
		ActivePlayers uint32
		Players       uint32
		Clicks        uint64
		AbilitiesUsed uint64
		ItemsUsed     uint64
	}
	var results []statEntry
	for i := c.nonEnded; i < c.invalid; i++ {
		if m[i].Err != nil {
			continue
		}
		r := m[i].Response
		if r.GetGameData().GetStatus() != messages.EMiniGameStatus_k_EMiniGameStatus_Running {
			continue
		}
		results = append(results, statEntry{
			ID:            i,
			Level:         r.GetGameData().GetLevel(),
			GameStart:     r.GetGameData().GetTimestampGameStart(),
			LevelStart:    r.GetGameData().GetTimestampLevelStart(),
			ActivePlayers: r.GetStats().GetNumActivePlayers(),
			Players:       r.GetStats().GetNumPlayers(),
			Clicks:        r.GetStats().GetNumClicks(),
			AbilitiesUsed: r.GetStats().GetNumAbilitiesActivated(),
			ItemsUsed:     r.GetStats().GetNumAbilityItemsActivated(),
		})
	}
	return json.Marshal(results)
}
