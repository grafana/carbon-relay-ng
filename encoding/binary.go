package encoding

// import (
// 	"bufio"
// 	"encoding/binary"
// 	"fmt"
// 	"io"
// 	"math"

// 	"github.com/graphite-ng/carbon-relay-ng/input"
// 	log "github.com/sirupsen/logrus"
// )

// // Internal Format is an kinda-optimized serialization to be used between relays
// type InternalFormatAdapter struct {
// }

// func (b *InternalFormatAdapter) Kind() string {
// 	return "internal"
// }

// func (b *InternalFormatAdapter) Handle(c io.Reader) error {
// 	r := bufio.NewReaderSize(c, 1024)
// 	log.Debug("Launching internal format handler")
// 	for {
// 		// Note that everything in this loop should proceed as fast as it can
// 		// so we're not blocked and can keep processing
// 		// so the validation, the pipeline initiated via dispatcher.Dispatch(), etc
// 		// must never block.
// 		var nameLenght uint8
// 		err := binary.Read(r, binary.BigEndian, &nameLenght)
// 		if err != nil {
// 			if err != io.EOF {
// 				return fmt.Errorf("couldn't metric name length: %s", err)
// 			}
// 			log.Debug("binary.go: error detecting metric name length. Message is finished")
// 			return nil
// 		}
// 		log.Debugf("binary.go: done detecting metric name length with binary.Read, length is %d", nameLenght)

// 		log.Debug("binary.go: reading metric name")
// 		name := make([]byte, nameLenght)
// 		err = binary.Read(r, binary.BigEndian, name)
// 		if err != nil {
// 			return fmt.Errorf("couldn't read metric name: %s", err)
// 		}
// 		var valueRaw uint64
// 		err = binary.Read(r, binary.BigEndian, valueRaw)
// 		if err != nil {
// 			return fmt.Errorf("can't read the datapoint float value for `%s`: %s", string(name), err)
// 		}
// 		value := math.Float64frombits(valueRaw)
// 		var time uint64
// 		time, err = binary.ReadUvarint(r)
// 		if err != nil {
// 			return fmt.Errorf("can't read the datapoint time value for `%s`: %s", string(name), err)
// 		}
// 		input.Dispatcher.DispatchMetric(Datapoint{Name: name, Timestamp: time, Value: value})
// 	}
// }
