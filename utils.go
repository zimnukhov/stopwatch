package main

import (
	"fmt"
	"time"
)

func millis(t time.Time) int64 {
	return t.UnixNano() / 1000000
}

func millisToTime(m int64) time.Time {
	return time.Unix(m/1000, (m%1000)*1000000)
}

const startMinute = 0

func dayStart(t time.Time, startHour int) time.Time {
	if t.Hour() < startHour || (t.Hour() == startHour && t.Minute() < startMinute) {
		t = t.Add(time.Hour * -24)
	}
	year, month, day := t.Date()
	return time.Date(year, month, day, startHour, startMinute, 0, 0, t.Location())
}

func dayEnd(t time.Time, startHour int) time.Time {
	t = t.Add(time.Hour * 24)
	return dayStart(t, startHour)
}

func apiDateFormat(t time.Time) string {
	return t.Format("2006-01-02")
}

func formatElapsedTime(t int64) string {
	sec := t / 1000

	h := sec / 3600
	m := (sec % 3600) / 60
	sec %= 60
	ms := t % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, ms)
}
