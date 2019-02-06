package destination

import (
	"bytes"
	"encoding/binary"

	ogorek "github.com/kisielk/og-rek"
	log "github.com/sirupsen/logrus"
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
		log.Fatal(err.Error())
	}
	messageBuf.Write(dataBuf.Bytes())
	return messageBuf.Bytes()
}
