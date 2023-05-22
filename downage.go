package main

import (
	"database/sql"
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
	Id         int           `json:"id"`
	StartTime  time.Time     `json:"start_time"`
	Duration   time.Duration `json:"duration"`
	Continuing bool          `json:"continuing"`
}

func initializeDatabase(dbPath string) (db *sql.DB, err error) {
	// Open the database
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create the table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS ping (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        startTime DATETIME,
        duration INTEGER,
        continuing INTEGER
    )`)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func randomizeList(list []string) []string {
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
	return list
}

func serverAvailable(server string) bool {
	fmt.Println("Pinging ", server)
	Command := fmt.Sprint("ping -c 1 ", server, "> /dev/null && echo true || echo false")
	output, err := exec.Command("/bin/sh", "-c", Command).Output()
	result := strings.TrimSpace(string(output))
	if err != nil {
		fmt.Println("Error pinging server: ", err)
		//log.Fatal(err)
	}
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

func updateContinuingRecord(db sql.DB, duration time.Duration) {
	// Get the last record
	//var lastRecord PingResult
	var id int
	var startTime time.Time
	var old_duration int
	var continuing bool
	err := db.QueryRow(`SELECT id, startTime, duration, continuing FROM ping ORDER BY startTime DESC LIMIT 1`).Scan(&id, &startTime, &old_duration, &continuing)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println("Error getting last record: ", err)
		log.Fatal(err)
	}

	// Update the last record
	_, err = db.Exec(`UPDATE ping SET duration = ?, continuing = ? WHERE id = ?`, duration.Milliseconds(), true, id)
	if err != nil {
		log.Fatal(err)
	}
}

func dumpAllRecordsAsJson(db sql.DB) {
	// Get all the records
	rows, err := db.Query(`SELECT * FROM ping`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Iterate over the records
	for rows.Next() {
		var record PingResult
		err = rows.Scan(&record.Id, &record.StartTime, &record.Duration, &record.Continuing)
		if err != nil {
			log.Fatal(err)
		}
		// dump the record as JSON to Stdout
		json.NewEncoder(os.Stdout).Encode(record)
	}
}

func main() {
	dbPath := flag.String("db", "./ping.db", "path to the SQLite database file")
	serverList := flag.String("servers", "8.8.8.8,1.1.1.1", "comma-separated list of ping servers")
	pollInterval := flag.Duration("poll", time.Second, "polling interval")
	dump := flag.Bool("dump", false, "dump the database and exit")

	flag.Parse()

	// Initialize the database
	db, err := initializeDatabase(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Dump the database and exit
	if *dump {
		dumpAllRecordsAsJson(*db)
		return
	}

	// convert the server list to an array
	servers := strings.Split(*serverList, ",")
	if len(servers) == 0 {
		log.Fatal("no servers specified")
	}

	continuing := false
	startTime := time.Now()

	// Ping loop
	for {

		currentTime := time.Now()
		internetIsUp := someServerAvailable(servers)

		// There are four cases:
		// 1. The internet is up and we're continuing
		// 2. The internet is up and we're not continuing
		// 3. The internet is down and we're continuing
		// 4. The internet is down and we're not continuing

		// Case 1: The internet is up and we're continuing
		if internetIsUp && continuing {
			fmt.Println("Case 1: The internet is up and we're continuing")
			// Update the last record
			updateContinuingRecord(*db, currentTime.Sub(startTime))
			continuing = false
		} else if internetIsUp && !continuing {
			fmt.Println("Case 2: The internet is up and we're not continuing")
			// Case 2: The internet is up and we're not continuing
			// Do nothing
		} else if !internetIsUp && continuing {
			fmt.Println("Case 3: The internet is down and we're continuing")
			// Case 3: The internet is down and we're continuing
			// Update the last record
			updateContinuingRecord(*db, currentTime.Sub(startTime))
		} else {
			// Case 4: The internet is down and we're not continuing
			fmt.Println("Case 4: The internet is down and we're not continuing")
			continuing = true
			// Insert a new record
			_, err = db.Exec(`INSERT INTO ping (startTime, duration, continuing)
								VALUES (?, ?, ?)`, startTime.UTC(), 0, continuing)
			if err != nil {
				log.Fatal(err)
			}
		}
		// Wait for a minute or until the next ping
		time.Sleep(*pollInterval)
	}

}
