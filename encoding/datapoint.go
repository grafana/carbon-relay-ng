package encoding

import (
	"fmt"
	"sort"
	"strconv"
)

type Datapoint struct {
	Name      string
	Timestamp uint64
	Value     float64
	Tags      Tags
}
type Tags map[string]string

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

// FullName returns the name and if present tags ordered, meant to provide a consistent way to identify
// a series since maps is not ordered not deterministic
func (dp Datapoint) FullName() string {
	if len(dp.Tags) > 0 {
		size := 0
		tags := make([]string, 0, len(dp.Tags))
		for tag, val := range dp.Tags {
			tags = append(tags, tag)
			size += len(tag) + len(val) + 2 // 2 for the `;` and `=`
		}
		sort.Strings(tags)
		buffer := make([]byte, size)
		pos := 0
		for i := range tags {
			pos += copy(buffer[pos:], ";")
			pos += copy(buffer[pos:], tags[i])
			pos += copy(buffer[pos:], "=")
			pos += copy(buffer[pos:], dp.Tags[tags[i]])
		}

		return fmt.Sprintf("%s%s", dp.Name, buffer)
	} else {
		return dp.Name
	}

}
