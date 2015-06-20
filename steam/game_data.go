package steam

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/zetafunction/steam-monster-game/messages"
	"io/ioutil"
	"log"
	"net/http"
)

const gameDataURL = "http://steamapi-a.akamaihd.net/ITowerAttackMiniGameService/GetGameData/v0001/?gameid=%d&include_stats=1&format=protobuf_raw"

type GameDataResult struct {
	Response *messages.CTowerAttack_GetGameData_Response
	Err      error
}

func (s *ApiService) GetGameData(id int) <-chan *GameDataResult {
	c := make(chan *GameDataResult)
	s.request <- func() {
		resp, err := http.Get(fmt.Sprintf(gameDataURL, id))
		if err != nil {
			log.Print("Get failed: ", err)
			c <- &GameDataResult{Err: err}
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Print("ReadAll failed: ", err)
			c <- &GameDataResult{Err: err}
			return
		}
		response := &messages.CTowerAttack_GetGameData_Response{}
		if err := proto.Unmarshal(body, response); err != nil {
			log.Print("Unmarshal failed:", err)
			log.Print("Response for ", id, " was ", body[:64])
			c <- &GameDataResult{Err: err}
			return
		}
		c <- &GameDataResult{Response: response}
	}
	return c
}
