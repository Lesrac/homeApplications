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
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	// Try full timestamp layout first, then fall back to date-only layout
	layouts := []string{
		"2006-01-02T15:04:05.000",
		"2006-01-02",
	}
	var parseErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, str)
		if err == nil {
			d.Time = t
			return nil
		}
		parseErr = err
	}
	return fmt.Errorf("failed to parse date: %w", parseErr)
}
