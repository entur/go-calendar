package calendar_test

import (
	"fmt"
	"time"

	"github.com/entur/go-calendar"
)

func ExampleCalendar_IsWorkday() {
	var c calendar.Calendar // weekends and statutory holidays off
	for _, s := range []string{"2026-12-24", "2026-12-25", "2026-12-28"} {
		d, _ := calendar.ParseDate(s)
		ok, err := c.IsWorkday(d)
		if err != nil {
			panic(err)
		}
		fmt.Println(s, ok)
	}
	// Output:
	// 2026-12-24 true
	// 2026-12-25 false
	// 2026-12-28 true
}

func ExampleCalendar_WithDaysOff() {
	c, err := calendar.Calendar{}.WithDaysOff(calendar.Julaften, calendar.Nyttarsaften)
	if err != nil {
		panic(err)
	}
	d, _ := calendar.ParseDate("2026-12-24")
	ok, _ := c.IsWorkday(d)
	fmt.Println(ok, c.DaysOff().Has(calendar.Julaften))
	// Output: false true
}

func ExampleCalendar_Observances() {
	var c calendar.Calendar
	d, _ := calendar.ParseDate("2027-05-17")
	set, _ := c.Observances(d)
	for o := range set.All() {
		fmt.Printf("%s (%s, %s)\n", o, o.Kind(), o.Basis())
	}
	// Output:
	// 17. mai (høytidsdag, lov om 1. og 17. mai som høgtidsdager § 1)
	// annen pinsedag (helligdag, helligdagsfredloven § 2 bokstav i)
}

// A kompass-style 28-day window ending on (and including) D.
func ExampleCalendar_CountWorkdays() {
	var c calendar.Calendar
	d, _ := calendar.ParseDate("2026-12-31")
	start, _ := d.AddDays(-27)
	end, _ := d.AddDays(1)
	window, err := calendar.NewRange(start, end)
	if err != nil {
		panic(err)
	}
	n, err := c.CountWorkdays(window)
	if err != nil {
		panic(err)
	}
	fmt.Println(start, "to", d, n)
	// Output: 2026-12-04 to 2026-12-31 19
}

func ExampleCalendar_Workdays() {
	var c calendar.Calendar
	from, _ := calendar.ParseDate("2027-03-24")
	to, _ := calendar.ParseDate("2027-04-01")
	r, _ := calendar.NewRange(from, to)
	days, err := c.Workdays(r)
	if err != nil {
		panic(err)
	}
	for d := range days {
		fmt.Println(d)
	}
	// Output:
	// 2027-03-24
	// 2027-03-30
	// 2027-03-31
}

func ExampleEaster() {
	e, _ := calendar.Easter(2019)
	fmt.Println(e) // the computus date, not the astronomical 2019-03-24
	// Output: 2019-04-21
}

func ExampleDateAt() {
	instant := time.Date(2026, time.November, 1, 23, 30, 0, 0, time.UTC)
	utc, _ := calendar.DateAt(instant, time.UTC)
	fmt.Println(utc)
	if _, err := calendar.DateAt(instant, nil); err != nil {
		fmt.Println(err)
	}
	// Output:
	// 2026-11-01
	// DateAt: nil location: calendar: invalid location
}

func ExampleParseDate() {
	if _, err := calendar.ParseDate("2026-02-30"); err != nil {
		fmt.Println(err)
	}
	// Output: parse "2026-02-30": calendar: invalid date
}

func ExampleObservance_DateIn() {
	d, _ := calendar.KristiHimmelfartsdag.DateIn(2027)
	fmt.Println(d)
	// Output: 2027-05-06
}
