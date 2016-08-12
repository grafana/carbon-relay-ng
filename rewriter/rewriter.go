package rewriter

import "bytes"
import "errors"

var errEmptyOld = errors.New("Rewriter must have non-empty 'old' specification")
var errMaxTooLow = errors.New("max must be >= -1. use -1 to mean no restriction")

type RW struct {
	Old string `json:"old"`
	New string `json:"new"`
	Max int    `json:"max"`
	old []byte
	new []byte
}

func NewFromByte(old, new []byte, max int) (RW, error) {
	if len(old) == 0 {
		return RW{}, errEmptyOld
	}
	if max < -1 {
		return RW{}, errMaxTooLow
	}
	return RW{
		Old: string(old),
		New: string(new),
		Max: max,
		old: old,
		new: new,
	}, nil
}

func New(old, new string, max int) (RW, error) {
	if len(old) == 0 {
		return RW{}, errEmptyOld
	}
	if max < -1 {
		return RW{}, errMaxTooLow
	}
	return RW{
		Old: old,
		New: new,
		Max: max,
		old: []byte(old),
		new: []byte(new),
	}, nil
}

// Do executes the rewriting of the metric line
// note: it allocates a new one, it would be better to replace in place.
func (r RW) Do(buf []byte) []byte {
	return bytes.Replace(buf, r.old, r.new, r.Max)
}
