package destination

import (
	"bytes"
	"encoding/binary"

	"go.uber.org/zap"

	ogorek "github.com/kisielk/og-rek"
)

func Pickle(dp *Datapoint) []byte {
	dataBuf := &bytes.Buffer{}
	pickler := ogorek.NewEncoder(dataBuf)

	// pickle format (in python talk): [(path, (timestamp, value)), ...]
	point := ogorek.Tuple{string(dp.Name), ogorek.Tuple{dp.Time, dp.Val}}
	list := []interface{}{point}
	pickler.Encode(list)
	messageBuf := &bytes.Buffer{}
	err := binary.Write(messageBuf, binary.BigEndian, uint32(dataBuf.Len()))
	if err != nil {
		zap.L().Fatal("pickle error", zap.Error(err))
	}
	messageBuf.Write(dataBuf.Bytes())
	return messageBuf.Bytes()
}
