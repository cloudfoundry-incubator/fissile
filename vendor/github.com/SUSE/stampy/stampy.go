package stampy

import (
	"encoding/csv"
	"os"
	"time"
)

// Stamp creates the csvFile should it not exist, and then append the
// specified event to its contents.
func Stamp(csvFilePath, originPath, timeSeriesName, event string) error {
	csvFile, err := os.OpenFile(csvFilePath,
		os.O_APPEND|os.O_WRONLY|os.O_CREATE,
		0666)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	w := csv.NewWriter(csvFile)
	err = w.Write([]string{
		time.Now().UTC().Format("2006-01-02 15:04:05"), // Excel-readable time format.
		originPath,
		timeSeriesName,
		event,
	})
	if err != nil {
		return err
	}

	w.Flush()
	return w.Error()
}
