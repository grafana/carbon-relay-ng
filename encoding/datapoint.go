package encoding

import "fmt"

type Datapoint struct {
	Name      string
	Timestamp uint64
	Value     float64
}

func (dp Datapoint) String() string {
	return fmt.Sprintf("%s %f %d", dp.Name, dp.Value, dp.Timestamp)
}
