//go:build ignore

// Command fetch_census downloads the US Census Bureau's "Easter Dates from
// 1600 to 2099" page and prints the Easter Sunday dates for 2000–2099 as
// normalised plain data, one YYYY-MM-DD per line. The SHA-256 of the HTML
// it parsed is printed to stderr so testdata/SOURCES.md can record it. Run
// from the repository root:
//
//	go run scripts/fetch_census.go > testdata/census/easter-2000-2099.txt
//
// Tests never run this program and never use the network.
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

const (
	pageURL   = "https://www.census.gov/data/software/x13as/genhol/easter-dates.html"
	firstYear = 2000
	lastYear  = 2099
)

// cell matches one date cell, e.g.
// <td headers="date-2000-5 y-2019">April 21</td>
var cell = regexp.MustCompile(`<td headers="date-\d{4}-\d y-(\d{4})">(March|April) (\d{1,2})</td>`)

func main() {
	if err := run(os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fetch_census:", err)
		os.Exit(1)
	}
}

func run(stdout, stderr io.Writer) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(pageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}

	dates := make(map[int]string)
	for _, m := range cell.FindAllSubmatch(page, -1) {
		year, _ := strconv.Atoi(string(m[1]))
		day, _ := strconv.Atoi(string(m[3]))
		month := 3
		if string(m[2]) == "April" {
			month = 4
		}
		if _, dup := dates[year]; dup {
			return fmt.Errorf("year %d listed twice", year)
		}
		dates[year] = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	}
	for year := firstYear; year <= lastYear; year++ {
		d, ok := dates[year]
		if !ok {
			return fmt.Errorf("year %d missing from the table", year)
		}
		if _, err := fmt.Fprintln(stdout, d); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stderr, "html-sha256 %x\n", sha256.Sum256(page))
	return err
}
