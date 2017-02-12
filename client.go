package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func countClientFlags() int {
	cnt := 0
	if *startFlag {
		cnt++
	}

	if *stopFlag {
		cnt++
	}

	if *statusFlag {
		cnt++
	}

	return cnt
}

func getURL(cfg *HTTPConfig) string {
	url := "http://localhost"
	if cfg.Port != 80 {
		url += fmt.Sprintf(":%d", cfg.Port)
	}

	url += cfg.HrefPrefix

	return url
}

func sendAPIRequest(url string) (*APIResponse, error) {
	resp, err := http.Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to stopwatch server: %s", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("read response: %s", err)
	}

	swData := &APIResponse{}
	err = json.Unmarshal(body, swData)

	if err != nil {
		return nil, fmt.Errorf("parse response: %s", err)
	}

	return swData, nil
}

func clientStart(baseURL string) (*APIResponse, error) {
	swData, err := sendAPIRequest(baseURL + "/start")
	return swData, err
}

func runClient(baseURL string) {
	var path string
	if *startFlag {
		path = "/start"
	} else if *stopFlag {
		path = "/stop"
	} else {
		path = "/time"
	}

	resp, err := sendAPIRequest(baseURL + path)

	if err != nil {
		fmt.Fprintf(os.Stderr, "stopwatch request failed: %s\n", err)
		return
	}

	msg := ""
	if resp.Running {
		msg += "Running"
	} else {
		msg += "Stopped"
	}

	msg += ". duration: "

	msg += formatElapsedTime(resp.Time)

	fmt.Println(msg)
}
