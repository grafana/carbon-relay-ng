package destination

import (
	"bytes"
	"encoding/binary"
	"github.com/kisielk/og-rek"
)

func pickle(dp *Datapoint) []byte {
	dataBuf := &bytes.Buffer{}
	pickler := ogórek.NewEncoder(dataBuf)

	// pickle format (in python talk): [(path, (timestamp, value)), ...]
	point := []interface{}{string(dp.Name), []interface{}{dp.Time, dp.Val}}
	list := []interface{}{point}
	pickler.Encode(list)
	messageBuf := &bytes.Buffer{}
	err := binary.Write(messageBuf, binary.BigEndian, uint32(dataBuf.Len()))
	if err != nil {
		log.Fatal(err.Error())
	}
	messageBuf.Write(dataBuf.Bytes())
	return messageBuf.Bytes()
}
