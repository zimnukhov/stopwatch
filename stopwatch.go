package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Stopwatch is the main type of stopwatch app.
// An instance of this type is created when the app starts.
// Structure holds all sessions for current day and start of the day.
type Stopwatch struct {
	ElapsedTime int64      // total ms for previous sessions
	Sessions    []*Session // closed sessions
	Session     *Session
	db          *sql.DB
	DayStart    time.Time
	config      *StopwatchConfig
	lock        sync.Mutex
}

// NewStopwatch creates and initializes a Stopwatch instance
// It opens log file, opens a database and loads sessions for current day
func NewStopwatch(cfg *Config) (*Stopwatch, error) {
	if cfg.Stopwatch.Log != "" {
		var logfd *os.File
		if _, err := os.Stat(cfg.Stopwatch.Log); os.IsNotExist(err) {
			logfd, err = os.Create(cfg.Stopwatch.Log)
			if err != nil {
				return nil, fmt.Errorf("failed to create log file: %s\n", err)

			}
		} else {
			logfd, err = os.OpenFile(cfg.Stopwatch.Log, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file: %s\n", err)
			}
		}
		log.SetOutput(logfd)
	}

	db, err := openDB(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to open db connection: %s\n", err)
	}

	sw := &Stopwatch{
		db:       db,
		DayStart: time.Now(),
		config:   cfg.Stopwatch,
	}

	err = sw.LoadSessions()

	if err != nil {
		return nil, fmt.Errorf("failed to load sessions: %s", err)
	}

	return sw, nil
}

// LoadSessions gets sessions for current day from db,
// populates s.Sessions slice and setes s.ElapsedTime
func (s *Stopwatch) LoadSessions() error {
	sessions, err := getAllSessions(s.db, s.config, s.DayStart)

	if err != nil {
		return fmt.Errorf("get sessions: %s", err)
	}

	for _, session := range sessions {
		if !session.Opened {
			s.ElapsedTime += session.Duration()
		} else {
			s.Session = session
		}
	}

	s.Sessions = sessions
	return nil
}

// Start starts stopwatch by opening a new session
func (s *Stopwatch) Start() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.Session == nil {
		s.Session = NewSession()

		err := s.Session.SaveOpened(s.db)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops stopwatch by closing current session
// and adds current session's duration to s.ElapsedTime
func (s *Stopwatch) Stop() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.Session != nil {
		session := s.Session
		session.Close()

		err := session.SaveClosed(s.db)
		if err != nil {
			return err
		}

		s.Sessions = append(s.Sessions, session)
		s.Session = nil
		s.ElapsedTime += session.Duration()
	}

	return nil
}

// APIResponse is returned in /time, /start and /stop handlers
type APIResponse struct {
	Time    int64  `json:"time"`
	Running bool   `json:"running"`
	Date    string `json:"date"`
}

// GetAPIResponse makes an APIResponse structure for current stopwatch instance
func (s *Stopwatch) GetAPIResponse() *APIResponse {
	total := int64(0)
	if s.Session != nil {
		total = s.ElapsedTime + time.Since(s.Session.Start).Nanoseconds()/1000000
	} else {
		total = s.ElapsedTime
	}

	return &APIResponse{
		Time:    total,
		Running: s.Session != nil,
		Date:    apiDateFormat(s.DayStart),
	}
}

func writeResponse(w http.ResponseWriter, sw *Stopwatch) error {
	resp := sw.GetAPIResponse()

	data, err := json.Marshal(resp)

	if err != nil {
		return err
	}

	_, err = w.Write(data)

	if err != nil {
		return err
	}

	return nil
}

// DaySplitWorker is a background worker that renews
// sessions slice of a Stopwatch instance when day ends
// if there is an open session in the end of day then it's split
// into two, first part is closed on the end of the day and
// the second part is starting at the same time
func DaySplitWorker(sw *Stopwatch, updates chan<- bool) {
	nextDayEnd := dayEnd(time.Now(), sw.config.DayStartHour)

	for {
		time.Sleep(time.Minute)

		if time.Now().After(nextDayEnd) {
			log.Printf("[split-worker] day ended")
			sw.lock.Lock()
			newSession, err := splitLastSession(sw.db, sw.config)
			if err != nil {
				log.Printf("[split-worker] failed to split last session: %s\n", err)
				sw.lock.Unlock()
				continue
			}

			sw.DayStart = dayStart(time.Now(), sw.config.DayStartHour)
			sw.Session = newSession
			sw.ElapsedTime = 0
			err = sw.LoadSessions()
			sw.lock.Unlock()
			if err != nil {
				log.Printf("[split-worker] failed to load sessions: %s\n", err)
				continue
			}
			nextDayEnd = dayEnd(time.Now(), sw.config.DayStartHour)
			updates <- true
		}
	}
}
