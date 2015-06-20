package poller

import (
	"encoding/json"
	"github.com/zetafunction/steam-monster-game/messages"
	"github.com/zetafunction/steam-monster-game/steam"
	"log"
	"sort"
	"time"
)

// TODO: Remember where the search started previously.
const searchStart = 48308

func findGameIndex(service *steam.APIService, start int) (int, error) {
	log.Print("Searching for invalid games starting at game ", start)
	lastValid := -1
	lastInvalid := -1
	errors := 0
	// Probe around the starting index to find a range to binary search over.
	for i, inc := start, 1; ; i, inc = i+inc, inc*2 {
		log.Print("new game scanner: probing game ", i)
		result := <-service.GetGameData(i)
		if result.Err != nil {
			log.Print("GetGameData failed: ", result.Err)
			if errors > 8 {
				log.Print("new game scanner: too many errors while finding next invalid game, giving up!")
				return 0, result.Err
			}
			errors++
		}
		switch result.Response.GetGameData().GetStatus() {
		case messages.EMiniGameStatus_k_EMiniGameStatus_Invalid:
			lastInvalid = i
			if lastValid == -1 {
				log.Print("new game scanner: initial index was invalid: searching downwards!")
				inc = -1
			}
		default:
			log.Print("Saw valid game at index ", i)
			lastValid = i
		}
		if lastValid != -1 && lastInvalid != -1 {
			break
		}
	}
	log.Printf("new game scanner: binary searching between valid game at %d and invalid game at %d", lastValid, lastInvalid)
	// Strictly speaking, a binary search is a bit dangerous because things might change.
	// Hopefully it returns close enough to the right result.
	var err error
	invalidOffset := sort.Search(lastInvalid-lastValid+1, func(i int) bool {
		// TODO: Panic?
		result := <-service.GetGameData(lastValid + i)
		if result.Err != nil {
			err = result.Err
		}
		return result.Response.GetGameData().GetStatus() == messages.EMiniGameStatus_k_EMiniGameStatus_Invalid
	})
	return lastValid + invalidOffset, err
}

type NewGameScanner struct {
	service *steam.APIService
	// The first invalid game ID. This may occasionally point to a valid game, since
	// the scanner scans 5 games ahead at a time.
	invalid int
	// If there are a lot of games in the waiting state, the new game scanner
	// sometimes has to temporarily increase the number of games to poll. The flex count
	// indicates the number of extra games that need to be polled at a given point.
	flex int

	DataUpdate        chan []byte
	InvalidGameUpdate chan int
}

func (s *NewGameScanner) Start() {
	go func() {
		for {
			if json, err := s.updateData(); err == nil {
				s.DataUpdate <- json
			} else {
				log.Print("updateData failed: ", err)
			}
			time.Sleep(time.Second)
		}
	}()
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
			result := <-s.service.GetGameData(i)
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
	firstInvalid := end
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
			if i < firstInvalid {
				firstInvalid = i
			}
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

	// The index of the first invalid game changed: try to find the next one.
	if s.invalid != firstInvalid {
		// Only update the index of the first invalid game if the Steam API is mostly working.
		if errors < 8 {
			log.Print("new game scanner: finding next invalid game...")
			nextInvalid, err := findGameIndex(s.service, firstInvalid-1)
			if err != nil {
				log.Print("findGameIndex failed: ", err)
			} else {
				log.Print("new game scanner: next invalid game at ", nextInvalid)
				firstInvalid = nextInvalid
			}
			s.flex += firstInvalid - s.invalid
			s.invalid = firstInvalid
		} else {
			log.Print("new game scanner: skipping invalid game search due to Steam errors")
		}
	}

	return json.Marshal(results)
}

func NewNewGameScanner(service *steam.APIService) (*NewGameScanner, error) {
	// TODO: This should probably be a receiver method of NewGameScanner.
	invalid, err := findGameIndex(service, searchStart)
	if err != nil {
		log.Print("findGameIndex failed: ", err)
		return nil, err
	}
	log.Print("First invalid game around ", invalid)
	p := &NewGameScanner{service, invalid, 25, make(chan []byte), make(chan int)}
	return p, nil
}
