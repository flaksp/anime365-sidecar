package anime365client

import (
	"errors"
	"time"
)

func ParseDateString(dateStr string) (time.Time, error) {
	if IsEmptyDateString(dateStr) {
		return time.Time{}, errors.New("date string is empty")
	}

	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}

	parsedDate, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr, location)
	if err != nil {
		return time.Time{}, err
	}

	return parsedDate, nil
}

func IsEmptyDateString(dateStr string) bool {
	return dateStr == "2000-01-01 00:00:00"
}
