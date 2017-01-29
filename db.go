package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func openDB(cfg *DBConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sql.Open("mysql", dsn)
	return db, err
}

// Session represents an interval when stopwatch was running
// when session is created, Start is set to current time and
// Opened is true. When it's closed End is set to current time
// and Opened to false
type Session struct {
	Start  time.Time
	End    time.Time
	Opened bool
}

// NewSession creates and opens a new session
func NewSession() *Session {
	return &Session{
		Start:  time.Now(),
		Opened: true,
	}
}

// SaveOpened saves session to db with End = NULL
func (s *Session) SaveOpened(db *sql.DB) error {
	startMillis := millis(s.Start)

	_, err := db.Exec("insert into sessions (start, end) values (?, NULL)", startMillis)
	if err != nil {
		return err
	}

	return nil
}

// Close changes session state to closed
func (s *Session) Close() {
	s.End = time.Now()
	s.Opened = false
}

// SaveClosed updates row for the session with end = current time
func (s *Session) SaveClosed(db *sql.DB) error {
	startMillis := millis(s.Start)
	endMillis := millis(s.End)

	_, err := db.Exec("update sessions set end = ? where start = ?", endMillis, startMillis)
	if err != nil {
		return err
	}

	return nil
}

// Duration returns session's duration in milliseconds
// it makes sense only for closed sessions
func (s *Session) Duration() int64 {
	return millis(s.End) - millis(s.Start)
}

// SessionAPIResponse is session representation in JSON
// it is returned in /sessions handler
// start and end are Unix timestamps in milliseconds
type SessionAPIResponse struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// ToAPIResponse converts session to an API response
// opened sessions get end = 0
func (s *Session) ToAPIResponse() SessionAPIResponse {
	resp := SessionAPIResponse{}
	resp.Start = millis(s.Start)
	if s.Opened {
		resp.End = 0
	} else {
		resp.End = millis(s.End)
	}
	return resp
}

func splitLastSession(db *sql.DB, cfg *StopwatchConfig) (*Session, error) {
	var lastStartSql sql.NullInt64
	var lastEndSql sql.NullInt64
	err := db.QueryRow("select start, end from sessions order by start desc limit 1").Scan(&lastStartSql, &lastEndSql)

	if !lastStartSql.Valid {
		return nil, fmt.Errorf("session start is Null")
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("select last session: %s", err)
	}

	if !lastEndSql.Valid {
		lastStart := lastStartSql.Int64
		autoEnd := dayStart(time.Now(), cfg.DayStartHour)
		log.Printf("open session started on %d will be ended on %d\n", lastStart, autoEnd)

		if lastStart < millis(autoEnd) {
			lastSession := &Session{
				Start: millisToTime(lastStart),
				End:   autoEnd,
			}

			err := lastSession.SaveClosed(db)

			if err != nil {
				return nil, err
			}

			lastSessionPart2 := &Session{
				Start: autoEnd,
			}

			err = lastSessionPart2.SaveOpened(db)

			if err != nil {
				return nil, err
			}

			return lastSessionPart2, nil
		}
	}

	return nil, nil
}

func getAllSessions(db *sql.DB, cfg *StopwatchConfig, t time.Time) ([]*Session, error) {
	start := dayStart(t, cfg.DayStartHour)
	end := dayEnd(t, cfg.DayStartHour)
	rows, err := db.Query("select start, end from sessions where start >= ? and (end <= ? or end is NULL) order by start", millis(start), millis(end))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var start int64
		var end int64
		rows.Scan(
			&start,
			&end, // end can be null!
		)

		session := &Session{
			Start: millisToTime(start),
		}

		if end == 0 {
			session.Opened = true
		} else {
			session.End = millisToTime(end)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DayStat represents a total time stopwatch was running for a specific date
// StartTime is a start of the date
// ElapsedTime is duration in milliseconds
type DayStat struct {
	StartTime   time.Time
	ElapsedTime int64
}

// Date returns formatted date for ui
func (ds DayStat) Date() string {
	return apiDateFormat(ds.StartTime)
}

// FormatElapsedTime returns formatted time for ui
func (ds DayStat) FormatElapsedTime() string {
	return formatElapsedTime(ds.ElapsedTime)
}

// ToAPIResponse converts DatStat instance to API response
func (ds DayStat) ToAPIResponse() DayStatAPIResponse {
	return DayStatAPIResponse{
		Time: ds.ElapsedTime,
		Date: ds.Date(),
	}
}

// DayStatAPIResponse is DayStat representation in JSON
// Date is formatted date
// Time is duration in milliseconds
type DayStatAPIResponse struct {
	Date string `json:"date"`
	Time int64  `json:"time"`
}

func loadDayStats(db *sql.DB, cfg *StopwatchConfig, from time.Time, to time.Time) ([]DayStat, error) {
	var stats []DayStat

	for t := from; t.Before(to); t = dayEnd(t, cfg.DayStartHour) {
		sessions, err := getAllSessions(db, cfg, t)
		if err != nil {
			return nil, err
		}
		stat := DayStat{
			StartTime: t,
		}
		for i := range sessions {
			if !sessions[i].Opened {
				stat.ElapsedTime += millis(sessions[i].End) - millis(sessions[i].Start)
			}
		}

		stats = append(stats, stat)
	}

	return stats, nil
}
