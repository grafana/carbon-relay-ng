package imperatives

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/aggregator"
	"github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/rewriter"
	"github.com/graphite-ng/carbon-relay-ng/route"
	tbl "github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/taylorchu/toki"
)

const (
	addBlack toki.Token = iota
	addAgg
	addRouteSendAllMatch
	addRouteSendFirstMatch
	addRouteConsistentHashing
	addRouteGrafanaNet
	addDest
	addRewriter
	delRoute
	modDest
	modRoute
	str
	sep
	sumFn
	avgFn
	num
	optPrefix
	optAddr
	optSub
	optRegex
	optFlush
	optReconn
	optPickle
	optSpool
	optTrue
	optFalse
	optBufSize
	optFlushMaxNum
	optFlushMaxWait
	optTimeout
	optSSLVerify
	word
)

// we should make sure we apply changes atomatically. e.g. when changing dest between address A and pickle=false and B with pickle=true,
// we should never half apply the change if one of them fails.

var tokens = []toki.Def{
	{Token: addBlack, Pattern: "addBlack"},
	{Token: addAgg, Pattern: "addAgg"},
	{Token: addRouteSendAllMatch, Pattern: "addRoute sendAllMatch"},
	{Token: addRouteSendFirstMatch, Pattern: "addRoute sendFirstMatch"},
	{Token: addRouteConsistentHashing, Pattern: "addRoute consistentHashing"},
	{Token: addRouteGrafanaNet, Pattern: "addRoute grafanaNet"},
	{Token: addDest, Pattern: "addDest"},
	{Token: addRewriter, Pattern: "addRewriter"},
	{Token: delRoute, Pattern: "delRoute"},
	{Token: modDest, Pattern: "modDest"},
	{Token: modRoute, Pattern: "modRoute"},
	{Token: optPrefix, Pattern: "prefix="},
	{Token: optAddr, Pattern: "addr="},
	{Token: optSub, Pattern: "sub="},
	{Token: optRegex, Pattern: "regex="},
	{Token: optFlush, Pattern: "flush="},
	{Token: optReconn, Pattern: "reconn="},
	{Token: optPickle, Pattern: "pickle="},
	{Token: optSpool, Pattern: "spool="},
	{Token: optTrue, Pattern: "true"},
	{Token: optFalse, Pattern: "false"},
	{Token: optBufSize, Pattern: "bufSize="},
	{Token: optFlushMaxNum, Pattern: "flushMaxNum="},
	{Token: optFlushMaxWait, Pattern: "flushMaxWait="},
	{Token: optTimeout, Pattern: "timeout="},
	{Token: optSSLVerify, Pattern: "sslverify="},
	{Token: str, Pattern: "\".*\""},
	{Token: sep, Pattern: "##"},
	{Token: sumFn, Pattern: "sum"},
	{Token: avgFn, Pattern: "avg"},
	{Token: num, Pattern: "[0-9]+( |$)"}, // unfortunately we need the 2nd piece cause otherwise it would match the first of ip addresses etc. this means we need to TrimSpace later
	{Token: word, Pattern: "[^ ]+"},
}

// note the two spaces between a route and endpoints
// match options can't have spaces for now. sorry
var errFmtAddBlack = errors.New("addBlack <prefix|sub|regex> <pattern>")
var errFmtAddAgg = errors.New("addAgg <sum|avg> <regex> <fmt> <interval> <wait>")
var errFmtAddRoute = errors.New("addRoute <type> <key> [prefix/sub/regex=,..]  <dest>  [<dest>[...]] where <dest> is <addr> [prefix/sub,regex,flush,reconn,pickle,spool=...]") // note flush and reconn are ints, pickle and spool are true/false. other options are strings
var errFmtAddRouteGrafanaNet = errors.New("addRoute GrafanaNet key [prefix/sub/regex]  addr apiKey schemasFile [spool=true/false sslverify=true/false bufSize=int flushMaxNum=int flushMaxWait=int timeout=int]")
var errFmtAddDest = errors.New("addDest <routeKey> <dest>") // not implemented yet
var errFmtAddRewriter = errors.New("addRewriter <old> <new> <max>")
var errFmtModDest = errors.New("modDest <routeKey> <dest> <addr/prefix/sub/regex=>") // one or more can be specified at once
var errFmtModRoute = errors.New("modRoute <routeKey> <prefix/sub/regex=>")           // one or more can be specified at once

func Apply(table *tbl.Table, cmd string) error {
	s := toki.NewScanner(tokens)
	s.SetInput(strings.Replace(cmd, "  ", " ## ", -1)) // token library skips whitespace but for us double space is significant
	t := s.Next()
	switch t.Token {
	case addAgg:
		return readAddAgg(s, table)
	case addBlack:
		return readAddBlack(s, table)
	case addDest:
		return errors.New("sorry, addDest is not implemented yet")
	case addRouteSendAllMatch:
		return readAddRoute(s, table, route.NewSendAllMatch)
	case addRouteSendFirstMatch:
		return readAddRoute(s, table, route.NewSendFirstMatch)
	case addRouteConsistentHashing:
		return readAddRouteConsistentHashing(s, table)
	case addRouteGrafanaNet:
		return readAddRouteGrafanaNet(s, table)
	case addRewriter:
		return readAddRewriter(s, table)
	case delRoute:
		return readDelRoute(s, table)
	case modDest:
		return readModDest(s, table)
	case modRoute:
		return readModRoute(s, table)
	default:
		return fmt.Errorf("unrecognized command %q", t.Value)
	}
}

func readAddAgg(s *toki.Scanner, table *tbl.Table) error {
	t := s.Next()
	if t.Token != sumFn && t.Token != avgFn {
		return errors.New("invalid function. need sum/avg")
	}
	fun := string(t.Value)

	if t = s.Next(); t.Token != word {
		return errors.New("need a regex string")
	}
	regex := string(t.Value)

	if t = s.Next(); t.Token != word {
		return errors.New("need a format string")
	}
	outFmt := string(t.Value)

	if t = s.Next(); t.Token != num {
		return errors.New("need an interval number")
	}
	interval, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
	if err != nil {
		return err
	}

	if t = s.Next(); t.Token != num {
		return errors.New("need a wait number")
	}
	wait, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
	if err != nil {
		return err
	}

	agg, err := aggregator.New(fun, regex, outFmt, uint(interval), uint(wait), table.In)
	if err != nil {
		return err
	}
	table.AddAggregator(agg)
	return nil
}

func readAddBlack(s *toki.Scanner, table *tbl.Table) error {
	prefix_pat := ""
	sub_pat := ""
	regex_pat := ""
	t := s.Next()
	if t.Token != word {
		return errFmtAddBlack
	}
	method := string(t.Value)
	switch method {
	case "prefix":
		if t = s.Next(); t.Token != word {
			return errFmtAddBlack
		}
		prefix_pat = string(t.Value)
	case "sub":
		if t = s.Next(); t.Token != word {
			return errFmtAddBlack
		}
		sub_pat = string(t.Value)
	case "regex":
		if t = s.Next(); t.Token != word {
			return errFmtAddBlack
		}
		regex_pat = string(t.Value)
	default:
		return errFmtAddBlack
	}

	m, err := matcher.New(prefix_pat, sub_pat, regex_pat)
	if err != nil {
		return err
	}
	table.AddBlacklist(m)
	return nil
}

func readAddRoute(s *toki.Scanner, table *tbl.Table, constructor func(key, prefix, sub, regex string, destinations []*destination.Destination) (route.Route, error)) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	prefix, sub, regex, err := readRouteOpts(s)
	if err != nil {
		return err
	}

	destinations, err := readDestinations(s, table, true)
	if err != nil {
		return err
	}
	if len(destinations) == 0 {
		return fmt.Errorf("must get at least 1 destination for route '%s'", key)
	}

	route, err := constructor(key, prefix, sub, regex, destinations)
	if err != nil {
		return err
	}
	table.AddRoute(route)
	return nil
}

func readAddRouteConsistentHashing(s *toki.Scanner, table *tbl.Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	prefix, sub, regex, err := readRouteOpts(s)
	if err != nil {
		return err
	}

	destinations, err := readDestinations(s, table, false)
	if err != nil {
		return err
	}
	if len(destinations) < 2 {
		return fmt.Errorf("must get at least 2 destination for route '%s'", key)
	}

	route, err := route.NewConsistentHashing(key, prefix, sub, regex, destinations)
	if err != nil {
		return err
	}
	table.AddRoute(route)
	return nil
}
func readAddRouteGrafanaNet(s *toki.Scanner, table *tbl.Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRouteGrafanaNet
	}
	key := string(t.Value)

	prefix, sub, regex, err := readRouteOpts(s)
	if err != nil {
		return err
	}

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteGrafanaNet
	}
	addr := string(t.Value)

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteGrafanaNet
	}
	apiKey := string(t.Value)

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteGrafanaNet
	}
	schemasFile := string(t.Value)
	t = s.Next()

	var spool bool
	sslVerify := true
	var bufSize = int(1e7)  // since a message is typically around 100B this is 1GB
	var flushMaxNum = 10000 // number of metrics
	var flushMaxWait = 500  // in ms
	var timeout = 2000      // in ms

	for ; t.Token != toki.EOF; t = s.Next() {
		switch t.Token {
		case optSpool:
			t = s.Next()
			if t.Token == optTrue || t.Token == optFalse {
				spool, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}
		case optBufSize:
			t = s.Next()
			if t.Token == num {
				bufSize, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}
		case optFlushMaxNum:
			t = s.Next()
			if t.Token == num {
				flushMaxNum, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}
		case optFlushMaxWait:
			t = s.Next()
			if t.Token == num {
				flushMaxWait, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}
		case optTimeout:
			t = s.Next()
			if t.Token == num {
				timeout, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}
		case optSSLVerify:
			t = s.Next()
			if t.Token == optTrue || t.Token == optFalse {
				sslVerify, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}

		default:
			return fmt.Errorf("unexpected token %d %q", t.Token, t.Value)
		}
	}

	route, err := route.NewGrafanaNet(key, prefix, sub, regex, addr, apiKey, schemasFile, spool, sslVerify, bufSize, flushMaxNum, flushMaxWait, timeout)
	if err != nil {
		return err
	}
	table.AddRoute(route)
	return nil
}

func readAddRewriter(s *toki.Scanner, table *tbl.Table) error {
	var t *toki.Result
	if t = s.Next(); t.Token != word {
		return errFmtAddRewriter
	}
	old := t.Value
	if t = s.Next(); t.Token != word {
		return errFmtAddRewriter
	}
	new := t.Value

	// token can be a word if it's a negative number. we should probably not have a separate number token, since numbers could be in so many formats
	// and we try out Atoi (or whatever fits) anyway.
	if t = s.Next(); t.Token != num && t.Token != word {
		return errFmtAddRewriter
	}
	max, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
	if err != nil {
		return errFmtAddRewriter
	}

	rw, err := rewriter.NewFromByte(old, new, max)
	if err != nil {
		return err
	}
	table.AddRewriter(rw)
	return nil
}

func readDelRoute(s *toki.Scanner, table *tbl.Table) error {
	t := s.Next()
	if t.Token != word {
		return errors.New("need route key")
	}
	key := string(t.Value)
	return table.DelRoute(key)
}

func readModDest(s *toki.Scanner, table *tbl.Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	t = s.Next()
	if t.Token != num {
		return errFmtAddRoute
	}
	index, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
	if err != nil {
		return err
	}

	opts := make(map[string]string)
	for {
		t = s.Next()
		switch t.Token {
		case toki.EOF:
			break
		case optAddr:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["addr"] = string(t.Value)
		case optPrefix:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["prefix"] = string(t.Value)
		case optSub:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["sub"] = string(t.Value)
		case optRegex:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["regex"] = string(t.Value)
		default:
			return errFmtModDest
		}
	}
	if len(opts) == 0 {
		return errors.New("modDest needs at least 1 option")
	}

	return table.UpdateDestination(key, index, opts)
}

func readModRoute(s *toki.Scanner, table *tbl.Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	opts := make(map[string]string)
	for {
		t = s.Next()
		switch t.Token {
		case toki.EOF:
			break
		case optPrefix:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["prefix"] = string(t.Value)
		case optSub:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["sub"] = string(t.Value)
		case optRegex:
			if t = s.Next(); t.Token != word {
				return errFmtModDest
			}
			opts["regex"] = string(t.Value)
		default:
			return errFmtModDest
		}
	}
	if len(opts) == 0 {
		return errors.New("modRoute needs at least 1 option")
	}

	return table.UpdateRoute(key, opts)
}

// we should read and apply all destinations at once,
// or at least make sure we apply them to the global datastruct at once,
// otherwise we can destabilize things / send wrong traffic, etc
func readDestinations(s *toki.Scanner, table *tbl.Table, allowMatcher bool) (destinations []*destination.Destination, err error) {
	for t := s.Next(); t.Token != toki.EOF; {
		if t.Token == sep {
			t = s.Next()
		}
		var prefix, sub, regex, addr, spoolDir string
		var spool, pickle bool
		flush := 1000
		reconn := 10000
		spoolDir = table.SpoolDir

		if t.Token != word {
			return destinations, errors.New("addr not set for endpoint")
		}
		addr = string(t.Value)

		for t.Token != toki.EOF && t.Token != sep {
			t = s.Next()
			switch t.Token {
			case optPrefix:
				if t = s.Next(); t.Token != word {
					return destinations, errFmtAddRoute
				}
				prefix = string(t.Value)
			case optSub:
				if t = s.Next(); t.Token != word {
					return destinations, errFmtAddRoute
				}
				sub = string(t.Value)
			case optRegex:
				if t = s.Next(); t.Token != word {
					return destinations, errFmtAddRoute
				}
				regex = string(t.Value)
			case optFlush:
				if t = s.Next(); t.Token != num {
					return destinations, errFmtAddRoute
				}
				flush, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return destinations, err
				}
			case optReconn:
				if t = s.Next(); t.Token != num {
					return destinations, errFmtAddRoute
				}
				reconn, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return destinations, err
				}
			case optPickle:
				if t = s.Next(); t.Token != optTrue && t.Token != optFalse {
					return destinations, errFmtAddRoute
				}
				pickle, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return destinations, fmt.Errorf("unrecognized pickle value '%s'", t)
				}
			case optSpool:
				if t = s.Next(); t.Token != optTrue && t.Token != optFalse {
					return destinations, errFmtAddRoute
				}
				spool, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return destinations, fmt.Errorf("unrecognized spool value '%s'", t)
				}
			case toki.EOF:
			case sep:
				break
			default:
				return destinations, fmt.Errorf("unrecognized option '%s'", t)
			}
		}

		periodFlush := time.Duration(flush) * time.Millisecond
		periodReConn := time.Duration(reconn) * time.Millisecond
		if !allowMatcher && (prefix != "" || sub != "" || regex != "") {
			return destinations, fmt.Errorf("matching options (prefix, sub, and regex) not allowed for this route type")
		}
		dest, err := destination.New(prefix, sub, regex, addr, spoolDir, spool, pickle, periodFlush, periodReConn)
		if err != nil {
			return destinations, err
		}
		destinations = append(destinations, dest)
	}
	return destinations, nil
}

func readRouteOpts(s *toki.Scanner) (prefix, sub, regex string, err error) {
	for {
		t := s.Next()
		switch t.Token {
		case toki.EOF:
			break
		case toki.Error:
			return "", "", "", errors.New("read the error token instead of one i recognize")
		case optPrefix:
			if t = s.Next(); t.Token != word {
				return "", "", "", errors.New("bad prefix option")
			}
			prefix = string(t.Value)
		case optSub:
			if t = s.Next(); t.Token != word {
				return "", "", "", errors.New("bad sub option")
			}
			sub = string(t.Value)
		case optRegex:
			if t = s.Next(); t.Token != word {
				return "", "", "", errors.New("bad regex option")
			}
			regex = string(t.Value)
		case sep:
			return
		default:
			return "", "", "", fmt.Errorf("unrecognized option '%s'", t.Value)
		}
	}
	return
}
