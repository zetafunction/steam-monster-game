package poller

import (
	"github.com/zetafunction/steam-monster-game/messages"
	"github.com/zetafunction/steam-monster-game/steam"
	"log"
	"sort"
	"time"
)

type RangeFinder struct {
	api  *steam.APIService
	quit chan struct{}

	// The ID of the first non-ended game.
	nonEnded          int
	nonEndedListeners map[chan int]struct{}

	// The ID of the first invalid game.
	invalid          int
	invalidListeners map[chan int]struct{}

	// Notifications that still have to be sent.
	pending map[chan int]int
}

func NewRangeFinder(api *steam.APIService) *RangeFinder {
	return &RangeFinder{
		api,
		make(chan struct{}),
		1,
		make(map[chan int]struct{}),
		1,
		make(map[chan int]struct{}),
		make(map[chan int]int),
	}
}

func (f *RangeFinder) Start() {
	t := time.After(time.Second)
	go func() {
		for {
			select {
			case <-t:
				func() {
					defer func() { t = time.After(time.Second) }()
					f.updateInvalid()
					f.updateNonEnded()
				}()
			case <-f.quit:
				// TODO: Close any open listeners here?
				return
			}
			f.notifyPending()
		}
	}()
}

func (f *RangeFinder) Stop() {
	close(f.quit)
}

func (f *RangeFinder) SubscribeInvalid() chan int {
	c := make(chan int)
	f.invalidListeners[c] = struct{}{}
	return c
}

func (f *RangeFinder) SubscribeNonEnded() chan int {
	c := make(chan int)
	f.nonEndedListeners[c] = struct{}{}
	return c
}

func (f *RangeFinder) Unsubscribe(c chan int) {
	delete(f.invalidListeners, c)
	delete(f.pending, c)
}

// TODO: Adding some indirection through a map and enum would reduce the copy-paste here.
func (f *RangeFinder) updateInvalid() {
	newInvalid, err := f.findGame(f.invalid, invalidGameFinder)
	if err != nil {
		log.Print("range finder: findGame: ", err)
		return
	}
	if newInvalid == f.invalid {
		return
	}
	log.Print("range finder: invalid game changed to ", newInvalid)
	for c := range f.invalidListeners {
		f.pending[c] = newInvalid
	}
	f.invalid = newInvalid
}

func (f *RangeFinder) updateNonEnded() {
	newNonEnded, err := f.findGame(f.nonEnded, nonEndedGameFinder)
	if err != nil {
		log.Print("range finder: findGame: ", err)
		return
	}
	if newNonEnded == f.nonEnded {
		return
	}
	log.Print("range finder: non-ended game changed to ", newNonEnded)
	for c := range f.nonEndedListeners {
		f.pending[c] = newNonEnded
	}
	f.nonEnded = newNonEnded
}

func (f *RangeFinder) notifyPending() {
	for c, i := range f.pending {
		select {
		case c <- i:
			delete(f.pending, c)
		}
	}
}

type finderFunc func(*steam.GameDataResult) bool

func invalidGameFinder(r *steam.GameDataResult) bool {
	return r.Response.GetGameData().GetStatus() == messages.EMiniGameStatus_k_EMiniGameStatus_Invalid
}

func nonEndedGameFinder(r *steam.GameDataResult) bool {
	return r.Response.GetGameData().GetStatus() != messages.EMiniGameStatus_k_EMiniGameStatus_Ended
}
func (f *RangeFinder) findGame(start int, finder finderFunc) (int, error) {
	log.Print("range finder: searching for games starting at ", start)
	end := start
	errors := 0
	// Exponentially probe upwards to start.
	for i, inc := start, 1; ; i, inc = i+inc, inc*2 {
		log.Print("range finder: probing game ", i)
		result := <-f.api.GetGameData(i)
		if result.Err != nil {
			log.Print("GetGameData failed: ", result.Err)
			if errors > 8 {
				log.Print("range finder: too many errors while probing, giving up!")
				return 0, result.Err
			}
			errors++
			continue
		}
		if finder(result) {
			end = i
			break
		}
		start = i
	}
	log.Print("range finder: binary searching between ", start, " and ", end)
	// Strictly speaking, a binary search is a bit dangerous because things might change.
	// Hopefully it returns close enough to the right result.
	offset := sort.Search(end-start, func(i int) bool {
		// TODO: Should this do the same error limiting that the previous loop does?
		result := <-f.api.GetGameData(start + i)
		if result.Err != nil {
			return false
		}
		return finder(result)
	})
	return start + offset, nil
}
