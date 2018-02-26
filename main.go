package main

import (
	"bufio"
	"bytes"
	"log"
	"strconv"
	"strings"
	"time"

	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

type FetchedRow struct {
	Field string  `json:"field"`
	Value float64 `json:"value"`
}

func main() {
	router := mux.NewRouter().StrictSlash(true)
	assetHandler := http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/")))
	router.PathPrefix("/assets/").Handler(assetHandler)
	router.HandleFunc("/", servePage)
	router.HandleFunc("/uploadFile", uploadFile).Methods("POST")
	router.HandleFunc("/fetchData/{variable}", fetchData)
	http.ListenAndServe(":8080", router)
}

func servePage(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, "index.html")
}

func fetchData(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	variable := vars["variable"]
	splits := strings.Split(variable, "_")
	field := splits[0]
	metric := "count(" + field + ")"
	urlType := ""
	if len(splits) > 1 {
		metric = splits[1]
	}
	if len(splits) > 2 {
		urlType = splits[2]
	}

	db, err := sql.Open("sqlite3", "./logAnalyzer.db")
	if err != nil {
		log.Printf("Couldn't connect to DB: %v", err)
		return
	}

	query := "SELECT " + field + ", " + metric + " from logAnalyzer"
	if urlType != "" {
		query = query + " where urlType=\"" + urlType + "\""
	}
	query = query + " group by " + field
	if field != "hour" {
		query = query + " order by " + metric + " DESC"
	} else {
		query = query + " order by " + field + " ASC"
	}

	log.Printf("query: %s", query)

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error while fetching rows: %v", err)
		return
	}

	var theseRows []FetchedRow
	for rows.Next() {
		thisRow := FetchedRow{}
		err = rows.Scan(&thisRow.Field, &thisRow.Value)
		if err != nil {
			log.Printf("Unexpected columns: %v", err)
			continue
		}
		theseRows = append(theseRows, thisRow)
	}

	returnJson := new(bytes.Buffer)
	json.NewEncoder(returnJson).Encode(theseRows)

	writer.Header().Set("Content-Type", "application/json")
	writer.Write(returnJson.Bytes())
}

func uploadFile(writer http.ResponseWriter, request *http.Request) {
	file, _, err := request.FormFile("logFile")
	if err != nil {
		log.Printf("Error while uploading file: %v", err)
		return
	}

	db, err := sql.Open("sqlite3", "./logAnalyzer.db")
	if err != nil {
		log.Printf("Couldn't create DB: %v", err)
		return
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS logAnalyzer (" +
		"uuid text PRIMARY KEY, url text NULL, urlType text NULL, ipaddress text NULL, " +
		"date text NULL, hour integer NULL, responseTime integer NULL)")
	if err != nil {
		log.Printf("Couldn't create table: %v", err)
		return
	}

	inserted := 0
	updated := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Split(line, " ")
		if len(words) <= 1 {
			continue
		}

		uuid := words[0]
		if words[1] == "Started" {
			url := words[3]
			url = url[1 : len(url)-1]
			urlType := "api"
			if strings.Contains(url, "assets") {
				urlType = "assets"
			}
			ipaddress := words[5]
			date := words[7]

			logTime, err := time.Parse(time.RFC3339, words[7]+"T"+words[8]+"Z")
			if err != nil {
				log.Printf("Error parsing time: %v", err)
				continue
			}
			hour := logTime.Hour()

			stmt, err := db.Prepare("INSERT INTO logAnalyzer (uuid, url, urlType, ipaddress, " +
				"date, hour) VALUES (?, ?, ?, ?, ?, ?)")
			if err != nil {
				log.Printf("Error preparing statement: %v", err)
				continue
			}

			_, err = stmt.Exec(uuid, url, urlType, ipaddress, date, hour)
			if err != nil {
				log.Printf("Error inserting row: %v", err)
				continue
			}
			inserted++
		} else if words[1] == "Completed" {
			re := words[5]
			re = re[:len(re)-2]
			responseTime, err := strconv.ParseInt(re, 10, 64)
			if err != nil {
				log.Printf("Error parsing response time: %v", err)
				continue
			}

			stmt, err := db.Prepare("UPDATE logAnalyzer SET responseTime = ? WHERE uuid = ?")
			if err != nil {
				log.Printf("Error preparing statement: %v", err)
				continue
			}

			_, err = stmt.Exec(responseTime, uuid)
			if err != nil {
				log.Printf("Error updating row: %v", err)
				continue
			}
			updated++
		}
	}
	log.Printf("Inserted: %d, Updated: %d", inserted, updated)
}
