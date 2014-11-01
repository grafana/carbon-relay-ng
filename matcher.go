package main

import (
	"bytes"
	"regexp"
)

type Matcher struct {
	Prefix   []byte
	Sub      []byte
	Regex    string
	regexObj *regexp.Regexp // compiled version of Regex
}

func NewMatcher(prefix, sub, regex string) (*Matcher, error) {
	match := Matcher{[]byte(prefix), []byte(sub), regex, nil}
	err := match.UpdateRegex(regex)
	if err != nil {
		return nil, err
	}
	return &match, nil
}

func (m *Matcher) UpdateRegex(regex string) error {
	if len(regex) == 0 {
		m.Regex = regex
		m.regexObj = nil
		return nil
	}
	obj, err := regexp.Compile(regex)
	if err != nil {
		return err
	}
	m.Regex = regex
	m.regexObj = obj
	return nil
}

func (m *Matcher) Match(s []byte) bool {
	if len(m.Prefix) > 0 && !bytes.HasPrefix(s, m.Prefix) {
		return false
	}
	if len(m.Sub) > 0 && !bytes.Contains(s, m.Sub) {
		return false
	}
	if m.regexObj != nil && !m.regexObj.Match(s) {
		return false
	}
	return true
}

func (m *Matcher) Snapshot() Matcher {
	prefix := make([]byte, len(m.Prefix))
	copy(prefix, m.Prefix)

	sub := make([]byte, len(m.Sub))
	copy(sub, m.Sub)

	obj, _ := regexp.Compile(m.Regex)

	return Matcher{prefix, sub, m.Regex, obj}
}
