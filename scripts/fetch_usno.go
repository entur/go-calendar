//go:build ignore

// Command fetch_usno downloads the US Naval Observatory's "Selected
// Christian Observances" for 2000–2099 and prints them as normalised plain
// data, one line per year:
//
//	YEAR GOOD-FRIDAY EASTER ASCENSION WHIT-SUNDAY
//
// with every date as YYYY-MM-DD. The API version is printed to stderr so
// testdata/SOURCES.md can record it. Run from the repository root:
//
//	go run scripts/fetch_usno.go > testdata/usno/observances-2000-2099.txt
//
// Tests never run this program and never use the network.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	apiURL    = "https://aa.usno.navy.mil/api/observances/christian?year=%d"
	firstYear = 2000
	lastYear  = 2099
)

type response struct {
	Error      string `json:"error"`
	APIVersion string `json:"apiversion"`
	Data       []struct {
		Observance string `json:"observance"`
		Year       int    `json:"year"`
		Month      int    `json:"month"`
		Day        int    `json:"day"`
	} `json:"data"`
}

func main() {
	if err := run(os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fetch_usno:", err)
		os.Exit(1)
	}
}

func run(stdout, stderr io.Writer) error {
	client := &http.Client{Timeout: 30 * time.Second}
	apiVersion := ""
	for year := firstYear; year <= lastYear; year++ {
		r, err := fetch(client, year)
		if err != nil {
			return fmt.Errorf("year %d: %w", year, err)
		}
		if apiVersion == "" {
			apiVersion = r.APIVersion
		} else if r.APIVersion != apiVersion {
			return fmt.Errorf("year %d: apiversion changed from %q to %q", year, apiVersion, r.APIVersion)
		}
		want := []string{"Good Friday", "Easter", "Ascension Day", "Whit Sunday -- Pentecost"}
		got := make(map[string]string, len(r.Data))
		for _, o := range r.Data {
			if o.Year != year {
				return fmt.Errorf("year %d: %s reported for year %d", year, o.Observance, o.Year)
			}
			got[o.Observance] = fmt.Sprintf("%04d-%02d-%02d", o.Year, o.Month, o.Day)
		}
		line := fmt.Sprintf("%04d", year)
		for _, name := range want {
			d, ok := got[name]
			if !ok {
				return fmt.Errorf("year %d: observance %q missing", year, name)
			}
			line += " " + d
		}
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
		time.Sleep(250 * time.Millisecond) // be polite to the service
	}
	_, err := fmt.Fprintf(stderr, "apiversion %s\n", apiVersion)
	return err
}

func fetch(client *http.Client, year int) (response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		r, err := fetchOnce(client, year)
		if err == nil {
			return r, nil
		}
		lastErr = err
	}
	return response{}, lastErr
}

func fetchOnce(client *http.Client, year int) (response, error) {
	resp, err := client.Get(fmt.Sprintf(apiURL, year))
	if err != nil {
		return response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return response{}, fmt.Errorf("HTTP %s", resp.Status)
	}
	var r response
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return response{}, err
	}
	if r.Error != "" {
		return response{}, errors.New(r.Error)
	}
	if len(r.Data) == 0 {
		return response{}, errors.New("no observances in response")
	}
	return r, nil
}
