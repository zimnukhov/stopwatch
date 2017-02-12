package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// TemplateData is a context for rendering HTML templates
type TemplateData struct {
	Days         []DayStat
	PrevDate     string
	NextDate     string
	ElapsedTime  string
	HrefPrefix   string
	DayStartHour int
}

type websocketClient struct {
	updates chan bool
}

// UpdatesWorker is a background worker that broadcasts update events of stopwatch
// to all the clients connected via web sockets
func UpdatesWorker(input <-chan bool, register <-chan *websocketClient, unregister <-chan *websocketClient) {
	clients := make(map[*websocketClient]bool)
	logPrefix := "[websocket-updates]"

	for {
		select {
		case <-input:
			for client := range clients {
				select {
				case client.updates <- true:
				default:
					log.Printf("%s failed to send, removing client", logPrefix)
					delete(clients, client)
					close(client.updates)
				}
			}
		case client := <-register:
			clients[client] = true
		case client := <-unregister:
			if _, ok := clients[client]; ok {
				delete(clients, client)
				close(client.updates)
			}
		}
	}
}

// config-related flags
var cfgPath = flag.String("config", "/usr/local/stopwatch/stopwatch.conf", "path to config file")
var defaultCfgFlag = flag.Bool("default-config", false, "print default config and exit")

// flags for CLI commands
var startFlag = flag.Bool("start", false, "[CLI] start time")
var stopFlag = flag.Bool("stop", false, "[CLI] stop time")
var statusFlag = flag.Bool("status", false, "[CLI] show current status and time")

func main() {
	flag.Parse()

	if *defaultCfgFlag {
		err := PrintDefaultConfig()

		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to print config")
		}
		return
	}

	cfg, err := ParseConfig(*cfgPath)
	if err != nil {
		log.Fatalf("failed to parse config: %s\n", err)
		return
	}

	cliFlags := countClientFlags()
	if cliFlags > 1 {
		fmt.Fprintf(os.Stderr, "Only one CLI flag must be set")
		return
	}

	if cliFlags == 1 {
		// client mode
		baseURL := getURL(cfg.HTTP)
		runClient(baseURL)
		return
	}
	// server mode

	sw, err := NewStopwatch(cfg)
	if err != nil {
		log.Fatalf("failed to initialize stopwatch: %s\n", err)
		return
	}

	log.Printf("started\n")

	// a channel of all updates to stopwatch state, input for websocket worker
	updates := make(chan bool)
	register := make(chan *websocketClient)
	unregister := make(chan *websocketClient)

	go DaySplitWorker(sw, updates)
	go UpdatesWorker(updates, register, unregister)

	http.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		err := writeResponse(w, sw)

		if err != nil {
			log.Printf("failed to write response: %s\n", err)
		}

	})

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		err := sw.Start()

		if err != nil {
			log.Printf("failed to start: %s\n", err)
			return
		}

		err = writeResponse(w, sw)

		if err != nil {
			log.Printf("failed to write response: %s\n", err)
		}

		updates <- true
	})

	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		err := sw.Stop()
		if err != nil {
			log.Printf("failed to stop: %s\n", err)
			return
		}

		err = writeResponse(w, sw)

		if err != nil {
			log.Printf("failed to write response: %s\n", err)
		}

		updates <- true
	})

	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		sessionsAPI := make([]SessionAPIResponse, 0)

		ts, ok := r.URL.Query()["time"]

		if !ok {
			for _, session := range sw.Sessions {
				sessionsAPI = append(sessionsAPI, session.ToAPIResponse())
			}

			if sw.Session != nil {
				sessionsAPI = append(sessionsAPI, sw.Session.ToAPIResponse())
			}
		} else {
			tsInt, err := strconv.ParseInt(ts[0], 10, 64)
			if err != nil {
				log.Printf("failed to parse time: %s\n", err)
				return
			}

			sessions, err := getAllSessions(sw.db, sw.config, millisToTime(tsInt))
			if err != nil {
				log.Printf("failed to load sessions: %s\n", err)
				return
			}

			for _, session := range sessions {
				sessionsAPI = append(sessionsAPI, session.ToAPIResponse())
			}
		}

		data, err := json.Marshal(sessionsAPI)
		if err != nil {
			log.Printf("failed to marshal sessions: %s\n", err)
			return
		}

		_, err = w.Write(data)
		if err != nil {
			log.Printf("failed to write response: %s\n", err)
		}
	})

	http.HandleFunc("/stat", func(w http.ResponseWriter, r *http.Request) {
		from := time.Now().Add(time.Hour * 24 * -7)
		days, err := loadDayStats(sw.db, sw.config, from, time.Now())
		if err != nil {
			log.Printf("failed to load day stats: %s\n", err)
			return
		}

		response := make([]DayStatAPIResponse, len(days))

		for i, ds := range days {
			response[i] = ds.ToAPIResponse()
		}

		data, err := json.Marshal(response)
		if err != nil {
			log.Printf("failed to marshal days stat: %s\n", err)
			return
		}

		_, err = w.Write(data)
		if err != nil {
			log.Printf("failed to write response: %s\n", err)
		}
	})

	dateRe := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})`)
	http.HandleFunc("/stats/", func(w http.ResponseWriter, r *http.Request) {
		splitURL := strings.Split(r.URL.Path, "/")
		if len(splitURL) < 3 {
			http.NotFound(w, r)
			return
		}

		matches := dateRe.FindStringSubmatch(splitURL[2])
		if matches == nil {
			http.NotFound(w, r)
			return
		}

		y, _ := strconv.Atoi(matches[1])
		m, _ := strconv.Atoi(matches[2])
		s, _ := strconv.Atoi(matches[3])

		from := time.Date(y, time.Month(m), s, sw.config.DayStartHour, 0, 0, 0, time.Now().Location())

		t, err := template.ParseFiles(path.Join(cfg.HTTP.StaticDir, "day_stat.html"))
		if err != nil {
			log.Printf("failed to parse day stat template file: %s\n", err)
			return
		}

		days, err := loadDayStats(sw.db, sw.config, from, from.Add(time.Hour*24))

		if len(days) == 0 {
			http.NotFound(w, r)
			return
		}

		err = t.Execute(w, TemplateData{
			HrefPrefix:   cfg.HTTP.HrefPrefix,
			ElapsedTime:  formatElapsedTime(days[0].ElapsedTime),
			DayStartHour: cfg.Stopwatch.DayStartHour,
		})

		if err != nil {
			log.Printf("failed to exec template: %s\n", err)
		}
	})

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	http.HandleFunc("/updates", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("failed to upgrade connection: %s", err)
			return
		}
		clientUpdates := make(chan bool)

		client := &websocketClient{
			updates: clientUpdates,
		}
		register <- client
		defer func() {
			unregister <- client
		}()

		for _ = range client.updates {
			err := conn.WriteMessage(websocket.TextMessage, []byte("update"))
			if err != nil {
				log.Printf("failed to write message: %s", err)
				return
			}
		}
	})

	http.Handle("/js/", http.FileServer(http.Dir(cfg.HTTP.StaticDir)))
	http.Handle("/css/", http.FileServer(http.Dir(cfg.HTTP.StaticDir)))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFiles(path.Join(cfg.HTTP.StaticDir, "stopwatch.html"))
		if err != nil {
			log.Printf("failed to parse main template file: %s\n", err)
			return
		}

		from := time.Now().Add(time.Hour * 24 * -7)
		days, err := loadDayStats(sw.db, sw.config, from, time.Now())
		if err != nil {
			log.Printf("failed to load day stats: %s\n", err)
			return
		}

		err = t.Execute(w, TemplateData{
			HrefPrefix:   cfg.HTTP.HrefPrefix,
			Days:         days,
			DayStartHour: cfg.Stopwatch.DayStartHour,
		})

		if err != nil {
			log.Printf("failed to exec template: %s\n", err)
		}
	})

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
