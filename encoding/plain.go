package encoding

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var (
	errFieldsNum              = errors.New("incorrect number of fields in metric")
	errTimestampFormatInvalid = errors.New("timestamp is not unix ts format")
	errValueInvalid           = errors.New("value is not int or float")
	errTwoConsecutiveDot      = "Two consecutive dots at %d and %d index"
	errBadTags                = "Bad tags : %s"
	errBadGraphiteTag         = "Bad graphiteTag : %s"
	errNoTags                 = "No tags"
	errBadMetricPath          = "Bad metricPath : %s"
	errEmptyline              = errors.New("empty line")
	errFmtNullInKey           = "null char at position %d"
	errFmtNotAscii            = "non-ascii char at position %d"
)

const PlainFormat FormatName = "plain"
const dotChar uint8 = '.'

type PlainAdapter struct {
	Validate bool
}

func NewPlain(validate bool) PlainAdapter {
	return PlainAdapter{Validate: validate}
}

func (p PlainAdapter) parseKey(firstPartDataPoint string) (string, error) {
	metricPath, err := parseMetricPath(firstPartDataPoint)
	if err != nil {
		return "", fmt.Errorf(errBadMetricPath, firstPartDataPoint)
	}

	return metricPath, nil
}
func putGraphiteTagInTags(metricPath string, firstPartDataPoint string, tags Tags) error {
	if len(metricPath) != len(firstPartDataPoint) {
		err := addGraphiteTagToTags(firstPartDataPoint[len(metricPath)+1:], tags)
		if err != nil {
			return fmt.Errorf(errBadTags, firstPartDataPoint)
		}
	}
	return nil
}

func parseMetricPath(key string) (string, error) {
	var previousChar uint8 = ' '
	i := 0
	for ; i < len(key) && key[i] != ';'; i++ {
		if key[i] == 0 {
			return "", fmt.Errorf(errFmtNullInKey, i)
		}
		if key[i] > unicode.MaxASCII {
			return "", fmt.Errorf(errFmtNotAscii, i)
		}
		if key[i] == dotChar && key[i] == previousChar {
			return "", fmt.Errorf(errTwoConsecutiveDot, i, i-1)
		}
		previousChar = key[i]
	}
	return key[:i], nil

}

//graphite tag follow the pattern key1=value1;key2=value2
func addGraphiteTagToTags(graphiteTag string, tags Tags) error {
	for index := 0; index < len(graphiteTag); index++ { //index++ is to remove the ; char for the next iteration
		//begin part to set the key
		tmp := strings.IndexByte(graphiteTag[index:], '=')
		if tmp == -1 {
			return fmt.Errorf(errBadGraphiteTag, graphiteTag)
		}
		equalsIndex := index + tmp
		if index >= equalsIndex {
			return fmt.Errorf(errBadGraphiteTag, graphiteTag)
		}
		key := graphiteTag[index:equalsIndex]
		//end part to set the key
		//begin part to set the value
		tmp = strings.IndexByte(graphiteTag[index:], ';')
		if tmp == -1 {
			index = len(graphiteTag) // at the end there is no ;
		} else {
			index = tmp + index
		}
		startValue := equalsIndex + 1
		if startValue >= index {
			return fmt.Errorf(errBadGraphiteTag, graphiteTag)
		}
		value := graphiteTag[startValue:index]
		//end part to set the value
		tags[key] = value
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

func (p PlainAdapter) Load(msgbuf []byte, tags Tags) (Datapoint, error) {
	return p.load(msgbuf, tags)
}

func (p PlainAdapter) load(msgbuf []byte, tags Tags) (Datapoint, error) {
	d := Datapoint{}
	if tags == nil {
		return d, fmt.Errorf(errNoTags)
	}
	if len(msgbuf) == 0 {
		return Datapoint{}, errEmptyline
	}
	msg := string(msgbuf)
	start := 0
	for msg[start] == ' ' {
		start++
	}
	if msg[start] == '.' {
		start++
	}
	firstSpace := strings.IndexByte(msg[start:], ' ')
	if firstSpace == -1 {
		return d, errFieldsNum
	}
	firstSpace += start
	var err error
	d.Name, err = p.parseKey(msg[start:firstSpace])
	if err != nil {
		return d, err
	}
	err = putGraphiteTagInTags(d.Name, msg[start:firstSpace], tags)
	if err != nil {
		return d, err
	}
	d.Tags = tags
	for msg[firstSpace] == ' ' {
		firstSpace++
	}
	nextSpace := strings.IndexByte(msg[firstSpace:], ' ')
	if nextSpace == -1 {
		return d, errFieldsNum
	}
	nextSpace += firstSpace
	v, err := strconv.ParseFloat(msg[firstSpace:nextSpace], 64)
	if err != nil {
		return d, err
	}
	d.Value = v
	for msg[nextSpace] == ' ' {
		nextSpace++
	}
	timestampLen := nextSpace
	for timestampLen < len(msgbuf) && msg[timestampLen] != ' ' {
		timestampLen++
	}

	ts, err := strconv.ParseUint(msg[nextSpace:timestampLen], 10, 32)
	if err != nil {
		return d, err
	}
	d.Timestamp = ts
	if err != nil {
		return d, err
	}
	return d, nil
}
