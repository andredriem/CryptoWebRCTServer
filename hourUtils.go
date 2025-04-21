// Validate if current hour is the same as the hourId or one hour earlier

package main

import (
	"fmt"
	"time"
)

func ValidateHourId(hourId int64) error {
	// Get the current UTC-0 hour
	currentHour := GetCurrentHour()

	// Check if the hourId is the same or one hour earlier
	if hourId != currentHour && hourId != currentHour-1 {
		return fmt.Errorf("hourId %d is not valid, it should be %d or %d", hourId, currentHour, currentHour-1)
	}
	return nil
}

func GetCurrentHour() int64 {
	// Get the current UTC-0 hour
	return time.Now().UTC().Unix() / 3600
}