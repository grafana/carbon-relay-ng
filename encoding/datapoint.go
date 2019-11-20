package encoding

import (
	"fmt"
	"strconv"
)

type Datapoint struct {
	Name      string
	Timestamp uint64
	Value     float64
	Metadata  map[string]string
}

func (dp Datapoint) String() string {
	return fmt.Sprintf("%s %f %d", dp.Name, dp.Value, dp.Timestamp)
}

func (dp Datapoint) AppendToBuf(buf []byte) (ret []byte) {
	ret = append(buf, dp.Name...)
	ret = append(ret, ' ')
	ret = strconv.AppendFloat(ret, dp.Value, 'f', 3, 64)
	ret = append(ret, ' ')
	ret = strconv.AppendUint(ret, dp.Timestamp, 10)
	return ret
}
