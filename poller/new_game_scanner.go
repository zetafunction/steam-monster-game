package poller

import (
	"encoding/json"
	"github.com/zetafunction/steam-monster-game/messages"
	"github.com/zetafunction/steam-monster-game/steam"
	"log"
	"time"
)

type NewGameScanner struct {
	api  *steam.APIService
	quit chan struct{}

	invalidUpdate chan int
	invalid       int
	// If there are a lot of games in the waiting state, the new game scanner
	// sometimes has to temporarily increase the number of games to poll. The flex count
	// indicates the number of extra games that need to be polled at a given point.
	flex int

	update chan []byte
}

func NewNewGameScanner(api *steam.APIService, finder *RangeFinder) *NewGameScanner {
	invalidUpdate := finder.SubscribeInvalid()
	return &NewGameScanner{
		api,
		make(chan struct{}),
		invalidUpdate,
		<-invalidUpdate,
		25,
		make(chan []byte)}
}

func (s *NewGameScanner) Start() {
	t := time.After(time.Second)
	go func() {
		for {
			select {
			case <-t:
				func() {
					defer func() { t = time.After(time.Second) }()
					json, err := s.updateData()
					if err != nil {
						log.Print("updateData failed: ", err)
						return
					}
					s.update <- json
				}()
			case newInvalid := <-s.invalidUpdate:
				s.flex += newInvalid - s.invalid
				s.invalid = newInvalid
			case <-s.quit:
				// TODO: Unsubscribe the invalid game listener.
				return
			}
		}
	}()
}

func (s *NewGameScanner) GetUpdateChannel() <-chan []byte {
	return s.update
}

func (s *NewGameScanner) Stop() {
	close(s.quit)
}

func (s *NewGameScanner) updateData() ([]byte, error) {
	log.Printf("new game scanner: updating (invalid: %d, flex: %d)\n", s.invalid, s.flex)
	start := s.invalid - 25 - s.flex
	end := s.invalid + 5

	type update struct {
		id     int
		result *steam.GameDataResult
	}
	c := make(chan update)
	requests := 0
	failed := 0
	for i := start; i < end; i++ {
		go func(i int) {
			result := <-s.api.GetGameData(i)
			if result.Err != nil {
				failed++
			}
			c <- update{i, result}
		}(i)
		requests++
	}
	m := make(map[int]*steam.GameDataResult)
	for requests > 0 {
		update := <-c
		m[update.id] = update.result
		requests--
	}

	type statusEntry struct {
		ID      int
		Status  string
		Players uint32
	}
	var results []statusEntry
	firstWaiting := end
	errors := 0
	for i := start; i < end; i++ {
		// Sometimes, the server likes to give out 500 errors, just because...
		if m[i].Err != nil {
			results = append(results, statusEntry{i, "???????", 0})
			errors++
			continue
		}
		var status string
		switch m[i].Response.GetGameData().GetStatus() {
		case messages.EMiniGameStatus_k_EMiniGameStatus_Invalid:
			status = "invalid"
		case messages.EMiniGameStatus_k_EMiniGameStatus_Running:
			status = "running"
		case messages.EMiniGameStatus_k_EMiniGameStatus_WaitingForPlayers:
			if i < firstWaiting {
				firstWaiting = i
			}
			status = "waiting"
		case messages.EMiniGameStatus_k_EMiniGameStatus_Ended:
			status = "ended"
		}
		results = append(results, statusEntry{
			i,
			status,
			m[i].Response.GetStats().GetNumPlayers(),
		})
	}

	// Always try to have at least one actively updated non-waiting entry.
	reclaimableFlex := firstWaiting - (start + 1)
	if reclaimableFlex > 0 && s.flex > 0 {
		s.flex -= reclaimableFlex
		if s.flex < 0 {
			s.flex = 0
		}
	}

	return json.Marshal(results)
}
