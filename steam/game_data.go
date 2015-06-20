package steam

import "encoding/json"
import "fmt"
import "io/ioutil"
import "net/http"
import "log"

const gameDataURL = "http://steamapi-a.akamaihd.net/ITowerAttackMiniGameService/GetGameData/v0001/?gameid=%d&include_stats=1&format=json"

// See the definition of EMiniGameStatus in
// http://cdn.akamai.steamstatic.com/steamcommunity/public/assets/minigame/towerattack/messages.proto
const (
	GameStatusInvalid = iota
	GameStatusWaiting
	GameStatusRunning
	GameStatusEnded
)

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
