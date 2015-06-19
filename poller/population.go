package poller

import "encoding/json"
import "time"
import "sort"
import "log"

// TODO: Remember where the search started previously.
const searchStart = 47000

func findGameIndex(start int) (int, error) {
	log.Print("Searching for invalid games starting at game ", start)
	lastValid := -1
	lastInvalid := -1
	// Probe around the starting index to find a range to binary search over.
	for i, inc := start, 1; ; i, inc = i+inc, inc*2 {
		log.Print("Probing game ", i)
		response, err := getGameData(i)
		if err != nil {
			// TODO: This should be more robust.
			log.Print("getGameData failed: ", err)
			return 0, err
		}
		switch response.GameData.Status {
		case gameStatusInvalid:
			lastInvalid = i
			if lastValid == -1 {
				log.Print("Initial index was invalid: searching downwards!")
				inc = -1
			}
		default:
			log.Print("Saw valid game at index ", i)
			lastValid = i
		}
		if lastValid != -1 && lastInvalid != -1 {
			break
		}
		time.Sleep(time.Millisecond * 250)
	}
	log.Printf("Binary searching between valid game at %d and invalid game at %d\n", lastValid, lastInvalid)
	// Strictly speaking, a binary search is a bit dangerous because things might change.
	// Hopefully it returns close enough to the right result.
	invalidOffset := sort.Search(lastInvalid-lastValid+1, func(i int) bool {
		// TODO: Panic?
		r, _ := getGameData(lastValid + i)
		time.Sleep(time.Millisecond * 250)
		return r.GameData.Status == gameStatusInvalid
	})
	return lastValid + invalidOffset, nil
}

type gameDataPoller struct {
	// The first invalid game ID. This may occasionally point to a valid game, since
	// the poller scans 5 games ahead at a time.
	invalid int
	// If there are a lot of games in the waiting state, the game data poller
	// sometimes has to temporarily increase the number of games to poll. The flex count
	// indicates the number of extra games that need to be polled at a given point.
	flex int
}

func (p *gameDataPoller) updateData() ([]byte, error) {
	log.Printf("Updating data (invalid: %d, flex: %d)\n", p.invalid, p.flex)
	start := p.invalid - 25 - p.flex
	end := p.invalid + 5

	type update struct {
		id    int
		reply *gameDataReply
	}
	c := make(chan update)
	requests := 0
	failed := 0
	for i := start; i < end; i++ {
		go func(i int) {
			reply, err := getGameData(i)
			if err != nil {
				failed++
			}
			c <- update{i, reply}
		}(i)
		requests++
	}
	m := make(map[int]*gameDataReply)
	for requests > 0 {
		update := <-c
		m[update.id] = update.reply
		requests--
	}

	type statusEntry struct {
		Id      int
		Status  string
		Players int
	}
	var results []statusEntry
	firstWaiting := end
	firstInvalid := end
	for i := start; i < end; i++ {
		// Sometimes, the server likes to give out 500 errors, just because...
		if m[i] == nil {
			results = append(results, statusEntry{i, "???????", -1})
			continue
		}
		var status string
		switch m[i].GameData.Status {
		case gameStatusInvalid:
			if i < firstInvalid {
				firstInvalid = i
			}
			status = "invalid"
		case gameStatusRunning:
			status = "running"
		case gameStatusWaiting:
			if i < firstWaiting {
				firstWaiting = i
			}
			status = "waiting"
		case gameStatusEnded:
			status = "ended"
		}
		results = append(results, statusEntry{i, status, m[i].Stats.NumPlayers})
	}

	// Always try to have at least one actively updated non-waiting entry.
	reclaimableFlex := firstWaiting - (start + 2)
	if reclaimableFlex > 0 && p.flex > 0 {
		p.flex -= reclaimableFlex
		if p.flex < 0 {
			p.flex = 0
		}
	}
	p.flex += firstInvalid - p.invalid
	p.invalid = firstInvalid

	return json.Marshal(results)
}

func Start() (<-chan []byte, error) {
	invalid, err := findGameIndex(searchStart)
	if err != nil {
		log.Print("findGameIndices failed: ", err)
		return nil, err
	}
	log.Print("First invalid game around ", invalid)
	p := &gameDataPoller{invalid, 0}
	c := make(chan []byte)
	go func() {
		for {
			t := time.NewTimer(time.Second)
			select {
			case <-t.C:
				if json, err := p.updateData(); err == nil {
					c <- json
				} else {
					log.Print("updateData failed: ", err)
				}
			}
		}
	}()
	return c, nil
}
