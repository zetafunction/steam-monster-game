package steam

import "encoding/json"
import "fmt"
import "io/ioutil"
import "net/http"
import "log"
import "time"

const gameDataURL = "http://steamapi-a.akamaihd.net/ITowerAttackMiniGameService/GetGameData/v0001/?gameid=%d&include_stats=1&format=json"

// See the definition of EMiniGameStatus in
// http://cdn.akamai.steamstatic.com/steamcommunity/public/assets/minigame/towerattack/messages.proto
const (
	GameStatusInvalid = iota
	GameStatusWaiting
	GameStatusRunning
	GameStatusEnded
)

const (
	maxRequestsInFlight  = 100
	maxRequestsPerSecond = 100
)

type ApiService struct {
	// Limiters for requests in flight and requests per second.
	inFlight  chan struct{}
	perSecond chan struct{}

	request         chan func()
	pendingRequests []func()

	quit chan struct{}
}

type GameDataReply struct {
	GameData struct {
		Status int
	} `json:"game_data"`
	Stats struct {
		NumPlayers int `json:"num_players"`
	}
	Err error
}

func (s *ApiService) GetGameData(id int) <-chan *GameDataReply {
	c := make(chan *GameDataReply)
	s.request <- func() {
		var response struct {
			Response GameDataReply
		}
		resp, err := http.Get(fmt.Sprintf(gameDataURL, id))
		if err != nil {
			log.Printf("Get failed: ", err)
			c <- &GameDataReply{Err: err}
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("ReadAll failed: ", err)
			c <- &GameDataReply{Err: err}
			return
		}
		if err := json.Unmarshal(body, &response); err != nil {
			log.Println("Unmarshal failed:", err)
			log.Printf("Response for %d was %s", id, body)
			c <- &GameDataReply{Err: err}
			return
		}
		c <- &response.Response
	}
	return c
}

func (s *ApiService) Start() {
	fillBucket(s.inFlight, maxRequestsInFlight)
	fillBucket(s.perSecond, maxRequestsPerSecond)
	t := time.NewTicker(time.Second)
	for {
		select {
		case <-t.C:
			fillBucket(s.perSecond, maxRequestsPerSecond)
			s.dispatchPendingRequests()

		case req := <-s.request:
			s.pendingRequests = append(s.pendingRequests, req)
			s.dispatchPendingRequests()
		case <-s.quit:
			return
		}
	}
}

func fillBucket(c chan<- struct{}, n int) {
	for i := 0; i < n; i++ {
		select {
		case c <- struct{}{}:
		default:
			return
		}
	}
}

func (s *ApiService) dispatchPendingRequests() {
	for len(s.pendingRequests) > 0 {
		select {
		case <-s.inFlight:
		default:
			return
		}
		select {
		case <-s.perSecond:
		default:
			s.inFlight <- struct{}{}
			return
		}
		req := s.pendingRequests[0]
		go func() {
			req()
			s.inFlight <- struct{}{}
		}()
		s.pendingRequests = s.pendingRequests[1:]
	}
}

func (s *ApiService) Stop() {
	close(s.quit)
}

func NewApiService() *ApiService {
	service := &ApiService{
		inFlight:  make(chan struct{}, maxRequestsInFlight),
		perSecond: make(chan struct{}, maxRequestsPerSecond),
		request:   make(chan func()),
		quit:      make(chan struct{}),
	}
	go service.Start()
	return service
}
