package encoding

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"unicode"
)

var (
	errFieldsNum              = errors.New("incorrect number of fields in metric")
	errTimestampFormatInvalid = errors.New("timestamp is not unix ts format")
	errValueInvalid           = errors.New("value is not int or float")
	errFmtNullInKey           = "null char at position %d"
	errFmtNotAscii            = "non-ascii char at position %d"
)

const PlainFormat FormatName = "plain"

type PlainAdapter struct {
	Validate bool
}

func NewPlain(validate bool) PlainAdapter {
	return PlainAdapter{Validate: validate}
}

func (p PlainAdapter) validateKey(key []byte) error {
	if p.Validate {
		for i := 0; i < len(key); i++ {
			if key[i] == 0 {
				return fmt.Errorf(errFmtNullInKey, i)
			}
			if key[i] > unicode.MaxASCII {
				return fmt.Errorf(errFmtNotAscii, i)
			}
		}
	}
	return nil
}

func (p PlainAdapter) KindS() string {
	return string(PlainFormat)
}

func (p PlainAdapter) Kind() FormatName {
	return PlainFormat
}

func (p PlainAdapter) Dump(dp Datapoint) []byte {
	return []byte(dp.String())
}

func (p PlainAdapter) Load(msg []byte) (Datapoint, error) {
	d := Datapoint{}

	start := 0
	for msg[start] == ' ' {
		start++
	}
	if msg[start] == '.' {
		start++
	}
	firstSpace := bytes.IndexByte(msg[start:], ' ')
	if firstSpace == -1 {
		return d, errFieldsNum
	}
	p.validateKey(msg[start:firstSpace])
	d.Name = string(msg[start:firstSpace])
	for msg[firstSpace] == ' ' {
		firstSpace++
	}
	nextSpace := bytes.IndexByte(msg[firstSpace:], ' ')
	if nextSpace == -1 {
		return d, errFieldsNum
	}
	nextSpace += firstSpace
	v, err := strconv.ParseFloat(string(msg[firstSpace:nextSpace]), 64)
	if err != nil {
		return d, err
	}
	d.Value = v
	for msg[nextSpace] == ' ' {
		nextSpace++
	}

	ts, err := strconv.ParseUint(string(msg[nextSpace:]), 10, 32)
	if err != nil {
		return d, err
	}
	d.Timestamp = ts
	return d, nil
}
