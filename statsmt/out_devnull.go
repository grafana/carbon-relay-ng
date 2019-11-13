package statsmt

import (
	"time"
)

func NewDevnull() {
	go func() {
		ticker := tick(time.Second)
		buf := make([]byte, 0)
		for now := range ticker {
			for _, metric := range Register.List() {
				metric.ReportGraphite(nil, buf[:], now)
			}
		}
	}()
}
