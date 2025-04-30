package models

import (
	"fmt"
	"time"
)

type AccessLevel string

const (
	Admin AccessLevel = "admin"
	User  AccessLevel = "user"
)

type AppUser struct {
	ID       int
	Name     string
	Access   AccessLevel
	Password string
}

type Action struct {
	UserID    int
	Action    string
	Timestamp DateOnly
}

type DateOnly struct {
	time.Time
}

/*
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	str := strings.Trim(string(b), `"`)
	parsedTime, err := time.Parse("2006-01-02T15:04:05.000", str)
	if err != nil {
		return err
	}
	ct.Time = parsedTime
	return nil
}
*/

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	// Remove the quotes around the date string
	str := string(b)
	str = str[1 : len(str)-1]

	// Parse the date string
	t, err := time.Parse("2006-01-02T15:04:05.000", str)
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}
	d.Time = t
	return nil
}
