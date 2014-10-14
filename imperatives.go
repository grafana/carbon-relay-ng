package main

import (
	"errors"
	"fmt"
	"github.com/taylorchu/toki"
	"strings"
)

const (
	addBlack toki.TokenType = iota
	addRouteSendAllMatch
	addRouteSendFirstMatch
	opt
	str
	word
)

// we should make sure we apply changes atomatically. e.g. when changing dest between address A and pickle=false and B with pickle=true,
// we should never half apply the change if one of them fails.

var tokenDefGlobal = []toki.TokenDef{
	{Type: addBlack, Pattern: "addBlack [a-z]+"},
	{Type: addRouteSendAllMatch, Pattern: "addRoute sendAllMatch [a-z]+"},
	{Type: addRouteSendFirstMatch, Pattern: "addRoute sendFirstMatch [a-z]+"},
	{Type: opt, Pattern: "[a-z]+="},
	{Type: str, Pattern: "\".*\""},
	{Type: word, Pattern: "[a-z]+"},
}

var tokenDefDest = []toki.TokenDef{
	{Type: opt, Pattern: "[a-z]+="},
	{Type: str, Pattern: "\".*\""},
	{Type: word, Pattern: "[a-z]+"},
}

// we should read and apply all destinations at once,
// or at least make sure we apply them to the global datastruct at once,
// otherwise we can destabilize things / send wrong traffic, etc
func readDestinations(specs []string) error {
	destinations := make([]Destination, 0, 0)
	s := toki.New(tokenDefDest)
	for _, spec := range specs {
		s.SetInput(spec)
		t := s.Next()
		if t.Type != str {
			return errors.New(fmt.Sprintf("expected destination endpoint spec, not '%s'", t))
		}
		// TODO validate spec!
		for {
			t := s.Next()
			if t.Type == opt {
				switch t.Value {
				case "sub=":
					sub := t.Next()
				case "prefix=":
					pref := t.Next()
				case "regex=":
					reg := t.Next()
				case "pickle=":
					t := s.Next()
					if t == "true" {
					} else if t == "false" {
					} else {
						return errors.New("unrecognized pickle value '" + t + "'")
					}
				case "spool=":
					t := s.Next()
					if t == "true" {
					} else if t == "false" {
					} else {
						return errors.New("unrecognized spool value '" + t + "'")
					}
				default:
					return errors.New("unrecognized option '" + t + "'")
				}
			} else {
				return errors.New("expected endpoint option, not '" + t + "'")
			}
		}
	}
	destinations = append(destinations, nil)
	return destinations
}

// note the two spaces between a route and endpoints
//"addBlack filter-out-all-metrics-matching-this-substring",
//"addRoute sendAllMatch carbon-default  127.0.0.1:2005 spool=false pickle=false",
//"addRoute sendFirstMatch demo sub=foo prefix=foo re=foo  127.0.0.1:12345 spool=true"

func applyCommand(table *Table, cmd string) error {
	inputs := strings.Split(cmd, "  ")
	s := toki.New(tokenDefGlobal)
	s.SetInput(input)
	t := s.Next()
	if t.Type == addBlack {
		split := strings.Split(t, " ")
		patt := split[1]
		if t = s.Next(); t.Type != toki.TokenEOF {
			return errors.New("extraneous input '" + t + "'")
		}
	} else if t.Type == addRouteSendAllMatch {
		split := strings.Split(t, " ")
		key := split[2]
		// read opts
		t = s.Next()
		for {
			t := s.Next()
			if t.Type == toki.TokenEOF {
				break
			}
			if t.Type == toki.TokenError {
				return errors.New(t)
			}
			if t.Type == opt {
				switch t.Value {
				case "sub=":
					"sub"
				case "prefix=":
				case "regex=":
				default:
					return errors.New("unrecognized option '" + t + "'")
				}
			}
		}
		if len(inputs) < 2 {
			// must get at least 1 endpoints
		}
		endpoints := readEndpoints(inputs[1:])
	} else if t.Type == addRouteSendFirstMatch {
	} else {
		return errors.New("unrecognized command '" + t + "'")
	}
	if t = s.Next(); t.Type != toki.TokenEOF {
		return errors.New("extraneous input '" + t + "'")
	}
}
