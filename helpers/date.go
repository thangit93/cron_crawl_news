package helpers

import (
	"fmt"
	"time"
)

func DiffDateToday(date string) (int, error) {
	// format dd/mm/yyyy
	layout := "02/01/2006"
	parsedDate, err := time.Parse(layout, date)
	if err != nil {
		fmt.Println("Lỗi parse ngày:", err)
		return 0, fmt.Errorf("lỗi parse ngày: %v", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	diff := 0
	if parsedDate.After(today) {
		diff = int(parsedDate.Sub(today).Hours() / 24)
	} else if parsedDate.Before(today) {
		diff = int(today.Sub(parsedDate).Hours() / 24)
	}
	return diff, nil
}
