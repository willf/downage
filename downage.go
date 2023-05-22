package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	dbPath = "./ping.db"
)

type PingResult struct {
	StartTime  time.Time `json:"start_time"`
	Duration   int64     `json:"duration"`
	Continuing bool      `json:"continuing"`
}

func (p PingResult) Dump() (err error) {
	// dump to Stdout as JSON
	err = json.NewEncoder(os.Stdout).Encode(p)
	return
}

func randomizeList(list []string) []string {
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
	return list
}

func serverAvailable(server string) bool {
	log.Print("Pinging ", server, "...")
	Command := fmt.Sprint("ping -c 1 ", server, "> /dev/null && echo true || echo false")
	output, err := exec.Command("/bin/sh", "-c", Command).Output()
	result := strings.TrimSpace(string(output))
	if err != nil {
		log.Println("Error pinging server: ", err)
		//log.Fatal(err)
	}
	//return false
	return result == "true"
}

func someServerAvailable(servers []string) bool {
	rServers := randomizeList(servers)
	for _, server := range rServers {
		if serverAvailable(server) {
			return true
		}
	}
	return false
}

func main() {
	log.SetOutput(os.Stderr)
	//dbPath := flag.String("db", "./ping.db", "path to the SQLite database file")
	serverList := flag.String("servers", "8.8.8.8,1.1.1.1", "comma-separated list of ping servers")
	pollInterval := flag.Duration("poll", time.Second*30, "polling interval")
	//dump := flag.Bool("dump", false, "dump the database and exit")

	flag.Parse()

	// convert the server list to an array
	servers := strings.Split(*serverList, ",")
	if len(servers) == 0 {
		log.Fatal("no servers specified")
	}

	continuing := false
	startTime := time.Now()

	// Ping loop
	for {
		internetIsUp := someServerAvailable(servers)

		// There are four cases:
		// 1. The internet is up and we're continuing
		// 2. The internet is up and we're not continuing
		// 3. The internet is down and we're continuing
		// 4. The internet is down and we're not continuing

		// Case 1: The internet is up and we're continuing
		if internetIsUp && continuing {
			log.Println("The internet is back up")

			continuing = false
			pingResult := PingResult{
				StartTime:  startTime.UTC(),
				Duration:   time.Since(startTime).Milliseconds(),
				Continuing: continuing,
			}
			pingResult.Dump()

		} else if internetIsUp && !continuing {
			log.Println("The internet is still up.")
			// Case 2: The internet is up and we're not continuing
			// Do nothing
		} else if !internetIsUp && continuing {
			log.Println("The internet is still down.")
			// Case 3: The internet is down and we're continuing
			pingResult := PingResult{
				StartTime:  startTime.UTC(),
				Duration:   time.Since(startTime).Milliseconds(),
				Continuing: continuing,
			}
			pingResult.Dump()
		} else {
			// Case 4: The internet is down and we're not continuing
			log.Println("The internet has gone down.")
			startTime = time.Now()
			continuing = true
			pingResult := PingResult{
				StartTime:  startTime.UTC(),
				Duration:   0,
				Continuing: continuing,
			}
			pingResult.Dump()
		}
		// Wait interval
		time.Sleep(*pollInterval)
	}

}
