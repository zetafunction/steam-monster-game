package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/zetafunction/steam-monster-game/messages"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strconv"
)

const shardPath = "[0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9]"
const shardFile = "[0-9][0-9][0-9][0-9][0-9].pb"

var includeBoringGames = flag.Bool("include-boring-games", false, "Include boring games in output.")

func main() {
	flag.Parse()
	if len(flag.Args()) != 1 {
		log.Fatal("proto2json expects a path to the directory of encoded protos")
	}

	pattern := filepath.Join(flag.Arg(0), shardPath, shardFile)
	matched, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatal("Glob failed: ", err)
	}

	unmarshalled := make(map[int]*messages.CTowerAttack_GetGameData_Response)
	for _, path := range matched {
		id, err := strconv.ParseInt(filepath.Base(path)[:5], 10, 0)
		if err != nil {
			log.Print("Failed to parse game ID out of ", path, ": ", err)
			continue
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			log.Print("Failed to read ", path, ": ", err)
			continue
		}
		response := &messages.CTowerAttack_GetGameData_Response{}
		err = proto.Unmarshal(data, response)
		if err != nil {
			log.Print("Failed to unmarshal ", path, ": ", err)
			continue
		}
		if response.GetGameData().GetLevel() == 0 && !*includeBoringGames {
			log.Print("Skipping boring game ", id)
			continue
		}
		// Trim out some fields that aren't interesting for ended games.
		response.GetGameData().Lanes = nil
		response.GetGameData().Events = nil
		unmarshalled[int(id)] = response
	}

	var ids []int
	for k := range unmarshalled {
		ids = append(ids, k)
	}
	sort.Ints(ids)

	type entry struct {
		ID       int                                         `json:"id"`
		Response *messages.CTowerAttack_GetGameData_Response `json:"response"`
	}
	var entries []entry
	for _, id := range ids {
		entries = append(entries, entry{id, unmarshalled[id]})
	}

	marshalled, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		log.Fatal("Marshal failed: ", err)
	}

	fmt.Print(string(marshalled))
}
