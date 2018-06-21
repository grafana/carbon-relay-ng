package rewriter

import (
	"bytes"
	"errors"
	"regexp"
)

var errEmptyOld = errors.New("Rewriter must have non-empty 'old' specification")
var errMaxTooLow = errors.New("max must be >= -1. use -1 to mean no restriction")
var errInvalidRegexp = errors.New("Invalid rewriter regular expression")
var errInvalidNotRegexp = errors.New("Invalid rewriter 'not' regular expression")
var errInvalidRegexpMax = errors.New("Regular expression rewriters require max to be -1")

// RW is a rewriter
type RW struct {
	Old   string `json:"old"`
	New   string `json:"new"`
	Not   string `json:"not"`
	Max   int    `json:"max"`
	old   []byte
	new   []byte
	not   []byte
	re    *regexp.Regexp
	notRe *regexp.Regexp
}

// NewFromByte creates a rewriter that will rewrite old to new, up to max times
// for regex, max must be -1
func New(old, new, not string, max int) (RW, error) {
	if len(old) == 0 {
		return RW{}, errEmptyOld
	}
	if max < -1 {
		return RW{}, errMaxTooLow
	}

	var re *regexp.Regexp
	var err error
	if len(old) > 1 && old[0:1] == "/" && old[len(old)-1:] == "/" {
		re, err = regexp.Compile(old[1 : len(old)-1])
		if err != nil {
			return RW{}, errInvalidRegexp
		}
		if max != -1 {
			return RW{}, errInvalidRegexpMax
		}
	}

	var notRe *regexp.Regexp
	if len(not) > 1 && not[0:1] == "/" && not[len(not)-1:] == "/" {
		notRe, err = regexp.Compile(not[1 : len(not)-1])
		if err != nil {
			return RW{}, errInvalidNotRegexp
		}
	}

	return RW{
		Old:   old,
		New:   new,
		Not:   not,
		Max:   max,
		old:   []byte(old),
		new:   []byte(new),
		not:   []byte(not),
		re:    re,
		notRe: notRe,
	}, nil
}

// Do executes the rewriting of the metric line
// note: it allocates a new one, it would be better to replace in place.
func (r RW) Do(buf []byte) []byte {
	if r.notRe != nil {
		if r.notRe.Match(buf) {
			return buf
		}
	} else if len(r.not) > 0 {
		if bytes.Contains(buf, r.not) {
			return buf
		}
	}
	if r.re != nil {
		return (*r.re).ReplaceAll(buf, r.new)
	}

	return bytes.Replace(buf, r.old, r.new, r.Max)
}
