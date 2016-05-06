package main

import (
	"errors"
	"fmt"
	"github.com/taylorchu/toki"
	"github.com/graphite-ng/carbon-relay-ng/aggregator"
	"strconv"
	"strings"
	"time"
	//"github.com/davecgh/go-spew/spew"
)

const (
	addBlack toki.Token = iota
	addAgg
	addRouteSendAllMatch
	addRouteSendFirstMatch
	addRouteConsistentHashing
	addDest
	modDest
	modRoute
	opt
	str
	word
)

// we should make sure we apply changes atomatically. e.g. when changing dest between address A and pickle=false and B with pickle=true,
// we should never half apply the change if one of them fails.

var tokenDefGlobal = []toki.Def{
	{Token: addBlack, Pattern: "addBlack .*"},
	{Token: addAgg, Pattern: "addAgg .*"},
	{Token: addRouteSendAllMatch, Pattern: "addRoute sendAllMatch [a-z-_]+"},
	{Token: addRouteSendFirstMatch, Pattern: "addRoute sendFirstMatch [a-z-_]+"},
	{Token: addRouteConsistentHashing, Pattern: "addRoute consistentHashing [a-z-_]+"},
	{Token: addDest, Pattern: "addDest [a-z-_]+"},
	{Token: modDest, Pattern: "modDest .*"},
	{Token: modRoute, Pattern: "modRoute .*"},
	{Token: opt, Pattern: "[a-z]+="},
	{Token: str, Pattern: "\".*\""},
	{Token: word, Pattern: "[^ ]+"},
}

var tokenDefDest = []toki.Def{
	{Token: opt, Pattern: "[a-z]+="},
	{Token: str, Pattern: "\".*\""},
	{Token: word, Pattern: "[^ ]+"},
}

// we should read and apply all destinations at once,
// or at least make sure we apply them to the global datastruct at once,
// otherwise we can destabilize things / send wrong traffic, etc
func readDestinations(specs []string, table *Table, allowMatcher bool) (destinations []*Destination, err error) {
	s := toki.NewScanner(tokenDefDest)
	for _, spec := range specs {
		//fmt.Println("spec" + spec)
		var prefix, sub, regex, addr, spoolDir string
		var spool, pickle bool
		flush := 1000
		reconn := 10000
		spoolDir = table.spoolDir
		s.SetInput(spec)
		t := s.Next()
		//fmt.Println("thisisit")
		if len(t.Value) == 0 {
			return destinations, errors.New("addr not set for endpoint")
		}
		addr = string(t.Value)
		//fmt.Println(t.Token, word)
		if t.Token != word {
			//fmt.Println("wtf", t.Token, word)
			return destinations, fmt.Errorf("expected destination endpoint spec, not '%s'", t.Value)
		}
		for {
			t := s.Next()
			if t.Token == opt {
				val := string(t.Value)
				//fmt.Println("yes got my opt with val", val)
				switch val {
				case "prefix=":
					val := s.Next()
					prefix = string(val.Value)
				case "sub=":
					val := s.Next()
					sub = string(val.Value)
				case "regex=":
					val := s.Next()
					regex = string(val.Value)
				case "flush=":
					val := s.Next()
					i, err := strconv.Atoi(string(val.Value))
					if err != nil {
						return destinations, err
					}
					flush = i
				case "reconn=":
					val := s.Next()
					i, err := strconv.Atoi(string(val.Value))
					if err != nil {
						return destinations, err
					}
					reconn = i
				case "pickle=":
					t := s.Next()
					val := string(t.Value)
					if val == "true" {
						pickle = true
					} else if val == "false" {
					} else {
						return destinations, fmt.Errorf("unrecognized pickle value '%s'", val)
					}
				case "spool=":
					t := s.Next()
					val := string(t.Value)
					if val == "true" {
						spool = true
					} else if val != "false" {
						return destinations, fmt.Errorf("unrecognized spool value '%s'", val)
					}
				default:
					return destinations, fmt.Errorf("unrecognized option '%s'", val)
				}
			} else {
				break
				//return destinations, errors.New(fmt.Sprintf("expected endpoint option, not token type %v with value '%s'", t.Token, t.Value))
			}
		}

		periodFlush := time.Duration(flush) * time.Millisecond
		periodReConn := time.Duration(reconn) * time.Millisecond
		if !allowMatcher && (prefix != "" || sub != "" || regex != "") {
			return destinations, fmt.Errorf("matching options (prefix, sub, and regex) not allowed for this route type")
		}
		dest, err := NewDestination(prefix, sub, regex, addr, spoolDir, spool, pickle, periodFlush, periodReConn)
		if err != nil {
			return destinations, err
		}
		destinations = append(destinations, dest)
	}
	return destinations, nil
}

// note the two spaces between a route and endpoints
//"addBlack [prefix|sub|regex] filter-out-all-metrics-matching-this-substring",
//"addRoute sendAllMatch carbon-default  127.0.0.1:2005 spool=false pickle=false",
//"addRoute sendFirstMatch demo sub=foo prefix=foo re=foo  127.0.0.1:12345 spool=true"
//addBlack string-without-spaces
//addRoute <type> <key> <match options>  <dests> # match options can't have spaces for now. sorry
//dests:
// <tcp addr> <options>

func applyCommand(table *Table, cmd string) error {
	inputs := strings.Split(cmd, "  ")
	s := toki.NewScanner(tokenDefGlobal)
	s.SetInput(inputs[0])
	t := s.Next()
	if t.Token == addBlack {
		inputs = strings.Fields(cmd)

		prefix_pat := ""
		sub_pat := ""
		regex_pat := ""

		if len(inputs) == 2 {
			// The fallback case to support the default substring method
			sub_pat = inputs[1]
		} else if len(inputs) == 3 {
			// New case supporting prefix, sub and regex patterns
			match_method := inputs[1]
			pattern := inputs[2]

			if match_method == "prefix" {
				prefix_pat = pattern
			} else if match_method == "sub" {
				sub_pat = pattern
			} else if match_method == "regex" {
				regex_pat = pattern
			} else {
				return errors.New("addBlack [prefix|sub|regex] <pattern> (invalid match type)")
			}
		} else {
			return errors.New("addBlack [prefix|sub|regex] <pattern>")
		}

		m, err := NewMatcher(prefix_pat, sub_pat, regex_pat)
		if err != nil {
			return err
		}
		table.AddBlacklist(m)

	} else if t.Token == addAgg {
		inputs = strings.Fields(cmd)
		if len(inputs) != 6 {
			return errors.New("addAgg <func> <match> <key> <interval> <wait>")
		}
		fun := inputs[1]
		regex := inputs[2]
		outFmt := inputs[3]
		interval, err := strconv.Atoi(inputs[4])
		if err != nil {
			return err
		}
		wait, err := strconv.Atoi(inputs[5])
		if err != nil {
			return err
		}
		agg, err := aggregator.New(fun, regex, outFmt, uint(interval), uint(wait), table.In)
		if err != nil {
			return err
		}
		table.AddAggregator(agg)
	} else if t.Token == addRouteSendAllMatch {
		split := strings.Split(string(t.Value), " ")
		key := split[2]
		if len(inputs) < 2 {
			return fmt.Errorf("must get at least 1 destination for route '%s'", key)
		}

		prefix, sub, regex, err := readRouteOpts(s)
		if err != nil {
			return err
		}
		destinations, err := readDestinations(inputs[1:], table, true)
		if err != nil {
			return err
		}
		route, err := NewRouteSendAllMatch(key, prefix, sub, regex, destinations)
		if err != nil {
			return err
		}
		table.AddRoute(route)
	} else if t.Token == addRouteSendFirstMatch {
		split := strings.Split(string(t.Value), " ")
		key := split[2]
		if len(inputs) < 2 {
			return fmt.Errorf("must get at least 1 destination for route '%s'", key)
		}

		prefix, sub, regex, err := readRouteOpts(s)
		if err != nil {
			return err
		}
		destinations, err := readDestinations(inputs[1:], table, true)
		if err != nil {
			return err
		}
		route, err := NewRouteSendFirstMatch(key, prefix, sub, regex, destinations)
		if err != nil {
			return err
		}
		table.AddRoute(route)
	} else if t.Token == addRouteConsistentHashing {
		split := strings.Split(string(t.Value), " ")
		key := split[2]
		if len(inputs) < 3 {
			return fmt.Errorf("must get at least 2 destinations for consistent hashing route '%s'", key)
		}

		prefix, sub, regex, err := readRouteOpts(s)
		if err != nil {
			return err
		}
		destinations, err := readDestinations(inputs[1:], table, false)
		if err != nil {
			return err
		}
		route, err := NewRouteConsistentHashing(key, prefix, sub, regex, destinations)
		if err != nil {
			return err
		}
		table.AddRoute(route)
	} else if t.Token == addDest {
		//split := strings.Split(string(t.Value), " ")
		//key := split[2]

		fmt.Println("val", t.Value)
		fmt.Println("inputs", inputs[0])
	} else if t.Token == modDest {
		split := strings.Split(string(t.Value), " ")
		if len(split) < 4 {
			return errors.New("need a key, index and at least one option")
		}
		key := split[1]
		index, err := strconv.Atoi(split[2])
		if err != nil {
			return err
		}
		opts := make(map[string]string)
		for _, str := range split[3:] {
			opt := strings.Split(str, "=")
			if len(opt) != 2 {
				return fmt.Errorf("bad option format at '%s'", str)
			}
			opts[opt[0]] = opt[1]
		}

		return table.UpdateDestination(key, index, opts)
	} else if t.Token == modRoute {
		split := strings.Split(string(t.Value), " ")
		if len(split) < 3 {
			return errors.New("need a key and at least one option")
		}
		key := split[1]
		opts := make(map[string]string)
		for _, str := range split[2:] {
			opt := strings.Split(str, "=")
			if len(opt) != 2 {
				return fmt.Errorf("bad option format at '%s'", str)
			}
			opts[opt[0]] = opt[1]
		}

		return table.UpdateRoute(key, opts)
	} else {
		return fmt.Errorf("unrecognized command '%s'", t.Value)
	}
	if t = s.Next(); t.Token != toki.EOF {
		//fmt.Println("h")
		return fmt.Errorf("extraneous input '%s'", t.Value)
	}
	return nil
}

func readRouteOpts(s *toki.Scanner) (prefix, sub, regex string, err error) {
	for {
		t := s.Next()
		//spew.Dump(t.Token)
		//spew.Dump(t)
		if t.Token == toki.EOF {
			break
		}
		if t.Token == toki.Error {
			return "", "", "", errors.New("read the error token instead of one i recognize")
		}
		if t.Token == opt {
			//fmt.Println("yet")
			switch string(t.Value) {
			case "prefix=":
				val := s.Next()
				prefix = string(val.Value)
			case "sub=":
				val := s.Next()
				sub = string(val.Value)
			case "regex=":
				val := s.Next()
				regex = string(val.Value)
			default:
				return "", "", "", fmt.Errorf("unrecognized option '%s'", t.Value)
			}
		}
	}
	return
}
