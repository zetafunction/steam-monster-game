package poller

import "encoding/json"
import "fmt"
import "io/ioutil"
import "net/http"
import "log"

const apiURL = "http://steamapi-a.akamaihd.net/ITowerAttackMiniGameService/GetGameData/v0001/?gameid=%d&include_stats=1&format=json"

// See the definition of EMiniGameStatus in
// http://cdn.akamai.steamstatic.com/steamcommunity/public/assets/minigame/towerattack/messages.proto
const (
	gameStatusInvalid = iota
	gameStatusWaiting
	gameStatusRunning
	gameStatusEnded
)

type gameDataReply struct {
	GameData struct {
		Status int
	} `json:"game_data"`
	Stats struct {
		NumPlayers int `json:"num_players"`
	}
}

func getGameData(id int) (*gameDataReply, error) {
	var response struct {
		Response gameDataReply
	}
	resp, err := http.Get(fmt.Sprintf(apiURL, id))
	if err != nil {
		log.Printf("Get failed: ", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ReadAll failed: ", err)
		return nil, err
	}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Println("Unmarshal failed:", err)
		log.Printf("Response for %d was %s", id, body)
		return nil, err
	}
	return &response.Response, nil
}
