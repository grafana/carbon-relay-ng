package rewriter

import "bytes"
import "errors"
import "regexp"

var errEmptyOld = errors.New("Rewriter must have non-empty 'old' specification")
var errMaxTooLow = errors.New("max must be >= -1. use -1 to mean no restriction")
var errInvalidRegexp = errors.New("Invalid rewriter regular expression")

type RW struct {
	Old string `json:"old"`
	New string `json:"new"`
	Max int    `json:"max"`
	old []byte
	new []byte
	re  *regexp.Regexp
}

func NewFromByte(old, new []byte, max int) (RW, error) {
	if len(old) == 0 {
		return RW{}, errEmptyOld
	}
	if max < -1 {
		return RW{}, errMaxTooLow
	}

	Old := string(old)

	var re *regexp.Regexp
	if Old[0:1] == "/" && Old[len(Old) - 1:] == "/" {
		var err error
		re, err = regexp.Compile(Old[1:len(Old) - 1])
		if err != nil {
			return RW{}, errInvalidRegexp
		}
	}

	return RW{
		Old: Old,
		New: string(new),
		Max: max,
		old: old,
		new: new,
		re:  re,
	}, nil
}

func New(old, new string, max int) (RW, error) {
	if len(old) == 0 {
		return RW{}, errEmptyOld
	}
	if max < -1 {
		return RW{}, errMaxTooLow
	}

	var re *regexp.Regexp
	if old[0:1] == "/" && old[len(old) - 1:] == "/" {
		var err error
		re, err = regexp.Compile(old[1:len(old) - 1])
		if err != nil {
			return RW{}, errInvalidRegexp
		}
	}

	return RW{
		Old: old,
		New: new,
		Max: max,
		old: []byte(old),
		new: []byte(new),
		re:  re,
	}, nil
}

// Do executes the rewriting of the metric line
// note: it allocates a new one, it would be better to replace in place.
func (r RW) Do(buf []byte) []byte {
	if r.re != nil {
		return (*r.re).ReplaceAll(buf, r.new)
	}

	return bytes.Replace(buf, r.old, r.new, r.Max)
}
