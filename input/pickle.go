package input

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	ogorek "github.com/kisielk/og-rek"
	log "github.com/sirupsen/logrus"
)

type Pickle struct {
	dispatcher Dispatcher
}

func NewPickle(dispatcher Dispatcher) *Pickle {
	return &Pickle{dispatcher}
}

func (p *Pickle) Kind() string {
	return "pickle"
}

func (p *Pickle) Handle(c io.Reader) error {
	r := bufio.NewReaderSize(c, 4096)
	// 500MB max payload size per pickle body
	maxLength := 500 * 1024 * 1024
	log.Debug("pickle.go: entering ReadLoop...")
	// ReadLoop
	for {

		// Note that everything in this loop should proceed as fast as it can
		// so we're not blocked and can keep processing
		// so the validation, the pipeline initiated via dispatcher.Dispatch(), etc
		// must never block.

		log.Debug("pickle.go: detecting payload length with binary.Read...")
		var length uint32
		err := binary.Read(r, binary.BigEndian, &length)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("couldn't read payload length: %s", err.Error())
			}
			log.Debug("pickle.go: EOF while detecting payload length")
			return nil
		}
		log.Debugf("pickle.go: done detecting payload length with binary.Read, length is %d", length)

		lengthTotal := int(length)
		if lengthTotal > maxLength {
			return fmt.Errorf("payload length of %d is more than the supported maximum %d", lengthTotal, maxLength)
		}

		prefix, err := r.Peek(3)
		if err != nil {
			return fmt.Errorf("couldn't read payload prefix: %s", err.Error())
		}

		// payload must start with opProto, <version>, opEmptyList
		if prefix[0] != '\x80' || prefix[2] != ']' {
			return errors.New("invalid payload prefix")
		}

		log.Debug("pickle.go: reading payload...")
		lengthRead := 0
		chunkLength := 4096
		if chunkLength > lengthTotal {
			chunkLength = lengthTotal
		}
		chunk := make([]byte, chunkLength, chunkLength)
		var payload bytes.Buffer
		for {
			toRead := lengthTotal - lengthRead
			if toRead > chunkLength {
				toRead = chunkLength
			}
			tmpLengthRead, err := r.Read(chunk[:toRead])
			if err != nil {
				return fmt.Errorf("couldn't read payload: %s", err.Error())
			}
			lengthRead += tmpLengthRead
			payload.Write(chunk[:tmpLengthRead])
			if lengthRead == lengthTotal {
				log.Debug("pickle.go: done reading payload")
				break
			}
		}

		decoder := ogorek.NewDecoder(&payload)

		log.Debug("pickle.go: decoding pickled data...")
		rawDecoded, err := decoder.Decode()
		if err != nil {
			if err != io.ErrUnexpectedEOF {
				return fmt.Errorf("error reading pickled data: %s", err.Error())
			}
			log.Debug("pickle.go: detected ErrUnexpectedEOF while decoding pickled data, nothing more to decode, breaking")
			return nil
		}
		log.Debug("pickle.go: done decoding pickled data")

		log.Debug("pickle.go: checking the type of pickled data...")
		decoded, ok := rawDecoded.([]interface{})
		if !ok {
			return fmt.Errorf("Unrecognized type %T for pickled data", rawDecoded)
		}
		log.Debug("pickle.go: done checking the type of pickled data")

		log.Debug("pickle.go: entering ItemLoop...")

	ItemLoop:
		for _, rawItem := range decoded {
			log.Debug("pickle.go: doing high-level validation of unpickled item and data...")
			item, ok := rawItem.(ogorek.Tuple)
			if !ok {
				log.Errorf("pickle.go: Unrecognized type %T for item", rawItem)
				p.dispatcher.IncNumInvalid()
				continue
			}
			if len(item) != 2 {
				log.Errorf("pickle.go: item length must be 2, got %d", len(item))
				p.dispatcher.IncNumInvalid()
				continue
			}

			metric, ok := item[0].(string)
			if !ok {
				log.Errorf("pickle.go: item metric must be a string, got %T", item[0])
				p.dispatcher.IncNumInvalid()
				continue
			}

			data, ok := item[1].(ogorek.Tuple)
			if !ok {
				log.Errorf("pickle.go: item data must be an array, got %T", item[1])
				p.dispatcher.IncNumInvalid()
				continue
			}
			if len(data) != 2 {
				log.Errorf("pickle.go: item data length must be 2, got %d", len(data))
				p.dispatcher.IncNumInvalid()
				continue
			}
			log.Debug("pickle.go: done doing high-level validation of unpickled item and data")

			var value string
			switch data[1].(type) {
			case string:
				value = data[1].(string)
			case uint8, uint16, uint32, uint64, int8, int16, int32, int64:
				value = fmt.Sprintf("%d", data[1])
			case float32, float64:
				value = fmt.Sprintf("%f", data[1])
			default:
				log.Errorf("pickle.go: Unrecognized type %T for value", data[1])
				p.dispatcher.IncNumInvalid()
				continue ItemLoop
			}

			var timestamp string
			switch data[0].(type) {
			case string:
				timestamp = data[0].(string)
			case uint8, uint16, uint32, uint64, int8, int16, int32, int64, (*big.Int):
				timestamp = fmt.Sprintf("%d", data[0])
			case float32, float64:
				timestamp = fmt.Sprintf("%.0f", data[0])
			default:
				log.Errorf("pickle.go: Unrecognized type %T for timestamp", data[0])
				p.dispatcher.IncNumInvalid()
				continue ItemLoop
			}

			buf := []byte(metric + " " + value + " " + timestamp)

			log.Debug("pickle.go: passing unpickled metric to dispatcher...")
			p.dispatcher.Dispatch(buf)

			log.Debug("pickle.go: exiting ItemLoop")
		}
		log.Debug("pickle.go: exiting ReadLoop")
	}
}
