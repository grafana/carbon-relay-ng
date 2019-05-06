package formats

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

type PlainHandler struct {
	Validate bool
}

func NewPlain(validate bool) PlainHandler {
	return PlainHandler{Validate: validate}
}

func (p PlainHandler) validateKey(key []byte) error {
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

func (p PlainHandler) KindS() string {
	return string(PlainFormat)
}

func (p PlainHandler) Kind() FormatName {
	return PlainFormat
}

func (p PlainHandler) Output(dp Datapoint) []byte {
	return []byte(fmt.Sprintf("%s %f %d", dp.Name, dp.Value, dp.Timestamp))
}

func (p PlainHandler) Process(msg []byte) (Datapoint, error) {
	d := Datapoint{}
	fields := bytes.Fields(msg)
	if len(fields) != 3 {
		return d, errFieldsNum
	}
	// Allow '.foo.bar' -> 'foo.bar'
	if len(fields[0]) != 0 && fields[0][0] == '.' {
		fields[0] = fields[0][1:]
	}
	if err := p.validateKey(fields[0]); err != nil {
		return d, err
	}
	d.Name = string(fields[0])
	v, err := strconv.ParseFloat(string(fields[1]), 64)
	if err != nil {
		return d, err
	}
	d.Value = v
	ts, err := strconv.ParseUint(string(fields[2]), 10, 32)
	if err != nil {
		return d, err
	}
	d.Timestamp = ts
	return d, nil
}
