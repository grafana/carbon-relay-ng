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
	"github.com/taylorchu/toki"
)

const (
	addBlack toki.Token = iota
	addAgg
	addRouteSendAllMatch
	addRouteSendFirstMatch
	addRouteConsistentHashing
	addRouteGrafanaNet
	addRouteKafkaMdm
	addDest
	addRewriter
	delRoute
	modDest
	modRoute
	str
	sep
	avgFn
	deltaFn
	deriveFn
	lastFn
	maxFn
	minFn
	stdevFn
	sumFn
	num
	optPrefix
	optAddr
	optCache
	optBlocking
	optSub
	optRegex
	optFlush
	optReconn
	optConnBufSize
	optIoBufSize
	optSpoolBufSize
	optSpoolMaxBytesPerFile
	optSpoolSyncEvery
	optSpoolSyncPeriod
	optSpoolSleep
	optUnspoolSleep
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
	optConcurrency
	optOrgId
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
	{Token: addRouteKafkaMdm, Pattern: "addRoute kafkaMdm"},
	{Token: addDest, Pattern: "addDest"},
	{Token: addRewriter, Pattern: "addRewriter"},
	{Token: delRoute, Pattern: "delRoute"},
	{Token: modDest, Pattern: "modDest"},
	{Token: modRoute, Pattern: "modRoute"},
	{Token: optPrefix, Pattern: "prefix="},
	{Token: optAddr, Pattern: "addr="},
	{Token: optCache, Pattern: "cache="},
	{Token: optBlocking, Pattern: "blocking="},
	{Token: optSub, Pattern: "sub="},
	{Token: optRegex, Pattern: "regex="},
	{Token: optFlush, Pattern: "flush="},
	{Token: optReconn, Pattern: "reconn="},
	{Token: optConnBufSize, Pattern: "connbuf="},
	{Token: optIoBufSize, Pattern: "iobuf="},
	{Token: optSpoolBufSize, Pattern: "spoolbuf="},
	{Token: optSpoolMaxBytesPerFile, Pattern: "spoolmaxbytesperfile="},
	{Token: optSpoolSyncEvery, Pattern: "spoolsyncevery="},
	{Token: optSpoolSyncPeriod, Pattern: "spoolsyncperiod="},
	{Token: optSpoolSleep, Pattern: "spoolsleep="},
	{Token: optUnspoolSleep, Pattern: "unspoolsleep="},
	{Token: optPickle, Pattern: "pickle="},
	{Token: optSpool, Pattern: "spool="},
	{Token: optTrue, Pattern: "true"},
	{Token: optFalse, Pattern: "false"},
	{Token: optBufSize, Pattern: "bufSize="},
	{Token: optFlushMaxNum, Pattern: "flushMaxNum="},
	{Token: optFlushMaxWait, Pattern: "flushMaxWait="},
	{Token: optTimeout, Pattern: "timeout="},
	{Token: optSSLVerify, Pattern: "sslverify="},
	{Token: optConcurrency, Pattern: "concurrency="},
	{Token: optOrgId, Pattern: "orgId="},
	{Token: str, Pattern: "\".*\""},
	{Token: sep, Pattern: "##"},
	{Token: avgFn, Pattern: "avg "},
	{Token: maxFn, Pattern: "max "},
	{Token: minFn, Pattern: "min "},
	{Token: sumFn, Pattern: "sum "},
	{Token: lastFn, Pattern: "last "},
	{Token: deltaFn, Pattern: "delta "},
	{Token: deriveFn, Pattern: "derive "},
	{Token: stdevFn, Pattern: "stdev "},
	{Token: num, Pattern: "[0-9]+( |$)"}, // unfortunately we need the 2nd piece cause otherwise it would match the first of ip addresses etc. this means we need to TrimSpace later
	{Token: word, Pattern: "[^ ]+"},
}

// note the two spaces between a route and endpoints
// match options can't have spaces for now. sorry
var errFmtAddBlack = errors.New("addBlack <prefix|sub|regex> <pattern>")
var errFmtAddAgg = errors.New("addAgg <avg|delta|derive|last|max|min|stdev|sum> [prefix/sub/regex=,..] <fmt> <interval> <wait> [cache=true/false]")
var errFmtAddRoute = errors.New("addRoute <type> <key> [prefix/sub/regex=,..]  <dest>  [<dest>[...]] where <dest> is <addr> [prefix/sub,regex,flush,reconn,pickle,spool=...]") // note flush and reconn are ints, pickle and spool are true/false. other options are strings
var errFmtAddRouteGrafanaNet = errors.New("addRoute grafanaNet key [prefix/sub/regex=,...]  addr apiKey schemasFile [spool=true/false sslverify=true/false blocking=true/false bufSize=int flushMaxNum=int flushMaxWait=int timeout=int concurrency=int orgId=int]")
var errFmtAddRouteKafkaMdm = errors.New("addRoute kafkaMdm key [prefix/sub/regex=,...]  broker topic codec schemasFile partitionBy orgId [blocking=true/false bufSize=int flushMaxNum=int flushMaxWait=int timeout=int]")
var errFmtAddDest = errors.New("addDest <routeKey> <dest>") // not implemented yet
var errFmtAddRewriter = errors.New("addRewriter <old> <new> <max>")
var errFmtModDest = errors.New("modDest <routeKey> <dest> <addr/prefix/sub/regex=>") // one or more can be specified at once
var errFmtModRoute = errors.New("modRoute <routeKey> <prefix/sub/regex=>")           // one or more can be specified at once

type Table interface {
	AddAggregator(agg *aggregator.Aggregator)
	AddRewriter(rw rewriter.RW)
	AddBlacklist(matcher *matcher.Matcher)
	AddRoute(route route.Route)
	DelRoute(key string) error
	UpdateDestination(key string, index int, opts map[string]string) error
	UpdateRoute(key string, opts map[string]string) error
	GetIn() chan []byte
	GetSpoolDir() string
}

func Apply(table Table, cmd string) error {
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
	case addRouteKafkaMdm:
		return readAddRouteKafkaMdm(s, table)
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

func readAddAgg(s *toki.Scanner, table Table) error {
	t := s.Next()
	if t.Token != sumFn && t.Token != avgFn && t.Token != minFn && t.Token != maxFn && t.Token != lastFn && t.Token != deltaFn && t.Token != deriveFn && t.Token != stdevFn {
		return errors.New("invalid function. need avg/max/min/sum/last/delta/derive/stdev")
	}
	fun := string(t.Value[:len(t.Value)-1]) // strip trailing space

	regex := ""
	prefix := ""
	sub := ""

	t = s.Next()
	// handle old syntax of a raw regex
	if t.Token == word {
		regex = string(t.Value)
		t = s.Next()
	}
	// scan for prefix/sub/regex=, stop when we hit a bare word (outFmt)
	for ; t.Token != toki.EOF && t.Token != word; t = s.Next() {
		switch t.Token {
		case optPrefix:
			if t = s.Next(); t.Token != word {
				return errFmtAddAgg
			}
			prefix = string(t.Value)
		case optSub:
			if t = s.Next(); t.Token != word {
				return errFmtAddAgg
			}
			sub = string(t.Value)
		case optRegex:
			if t = s.Next(); t.Token != word {
				return errFmtAddAgg
			}
			regex = string(t.Value)
		default:
			return fmt.Errorf("unexpected token %d %q", t.Token, t.Value)
		}
	}

	if regex == "" {
		return errors.New("need a regex string")
	}

	if t.Token != word {
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

	cache := true

	t = s.Next()
	for ; t.Token != toki.EOF; t = s.Next() {
		switch t.Token {
		case optCache:
			t = s.Next()
			if t.Token == optTrue || t.Token == optFalse {
				cache, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddAgg
			}
		default:
			return fmt.Errorf("unexpected token %d %q", t.Token, t.Value)
		}
	}

	agg, err := aggregator.New(fun, regex, prefix, sub, outFmt, cache, uint(interval), uint(wait), table.GetIn())
	if err != nil {
		return err
	}

	table.AddAggregator(agg)
	return nil
}

func readAddBlack(s *toki.Scanner, table Table) error {
	prefix := ""
	sub := ""
	regex := ""
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
		prefix = string(t.Value)
	case "sub":
		if t = s.Next(); t.Token != word {
			return errFmtAddBlack
		}
		sub = string(t.Value)
	case "regex":
		if t = s.Next(); t.Token != word {
			return errFmtAddBlack
		}
		regex = string(t.Value)
	default:
		return errFmtAddBlack
	}

	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return err
	}
	table.AddBlacklist(m)
	return nil
}

func readAddRoute(s *toki.Scanner, table Table, constructor func(key, prefix, sub, regex string, destinations []*destination.Destination) (route.Route, error)) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	prefix, sub, regex, err := readRouteOpts(s)
	if err != nil {
		return err
	}

	destinations, err := readDestinations(s, table, true, key)
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

func readAddRouteConsistentHashing(s *toki.Scanner, table Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	prefix, sub, regex, err := readRouteOpts(s)
	if err != nil {
		return err
	}

	destinations, err := readDestinations(s, table, false, key)
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
func readAddRouteGrafanaNet(s *toki.Scanner, table Table) error {
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

	var spool, blocking bool
	sslVerify := true
	var bufSize = int(1e7)  // since a message is typically around 100B this is 1GB
	var flushMaxNum = 10000 // number of metrics
	var flushMaxWait = 500  // in ms
	var timeout = 5000      // in ms
	var concurrency = 10    // number of concurrent connections to tsdb-gw
	var orgId = 1

	for ; t.Token != toki.EOF; t = s.Next() {
		switch t.Token {
		case optBlocking:
			t = s.Next()
			if t.Token == optTrue || t.Token == optFalse {
				blocking, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteGrafanaNet
			}
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
		case optConcurrency:
			t = s.Next()
			if t.Token == num {
				concurrency, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
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
		case optOrgId:
			t = s.Next()
			if t.Token == num {
				orgId, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
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

	route, err := route.NewGrafanaNet(key, prefix, sub, regex, addr, apiKey, schemasFile, spool, sslVerify, blocking, bufSize, flushMaxNum, flushMaxWait, timeout, concurrency, orgId)
	if err != nil {
		return err
	}
	table.AddRoute(route)
	return nil
}
func readAddRouteKafkaMdm(s *toki.Scanner, table Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRouteKafkaMdm
	}
	key := string(t.Value)

	prefix, sub, regex, err := readRouteOpts(s)
	if err != nil {
		return err
	}

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteKafkaMdm
	}
	brokers := strings.Split(string(t.Value), ",")

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteKafkaMdm
	}
	topic := string(t.Value)

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteKafkaMdm
	}
	codec := string(t.Value)
	if codec != "none" && codec != "gzip" && codec != "snappy" {
		return errFmtAddRouteKafkaMdm
	}

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteKafkaMdm
	}
	schemasFile := string(t.Value)

	t = s.Next()
	if t.Token != word {
		return errFmtAddRouteKafkaMdm
	}
	partitionBy := string(t.Value)
	if partitionBy != "byOrg" && partitionBy != "bySeries" {
		return errFmtAddRouteKafkaMdm
	}

	t = s.Next()
	if t.Token != word && t.Token != num {
		return errFmtAddRouteKafkaMdm
	}
	orgId, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
	if err != nil {
		return errFmtAddRouteKafkaMdm
	}

	var bufSize = int(1e7)  // since a message is typically around 100B this is 1GB
	var flushMaxNum = 10000 // number of metrics
	var flushMaxWait = 500  // in ms
	var timeout = 2000      // in ms
	var blocking = false

	t = s.Next()
	for ; t.Token != toki.EOF; t = s.Next() {
		switch t.Token {
		case optBlocking:
			t = s.Next()
			if t.Token == optTrue || t.Token == optFalse {
				blocking, err = strconv.ParseBool(string(t.Value))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteKafkaMdm
			}
		case optBufSize:
			t = s.Next()
			if t.Token == num {
				bufSize, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteKafkaMdm
			}
		case optFlushMaxNum:
			t = s.Next()
			if t.Token == num {
				flushMaxNum, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteKafkaMdm
			}
		case optFlushMaxWait:
			t = s.Next()
			if t.Token == num {
				flushMaxWait, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteKafkaMdm
			}
		case optTimeout:
			t = s.Next()
			if t.Token == num {
				timeout, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
				if err != nil {
					return err
				}
			} else {
				return errFmtAddRouteKafkaMdm
			}
		default:
			return fmt.Errorf("unexpected token %d %q", t.Token, t.Value)
		}
	}

	route, err := route.NewKafkaMdm(key, prefix, sub, regex, topic, codec, schemasFile, partitionBy, brokers, bufSize, orgId, flushMaxNum, flushMaxWait, timeout, blocking)
	if err != nil {
		return err
	}
	table.AddRoute(route)
	return nil
}

func readAddRewriter(s *toki.Scanner, table Table) error {
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

func readDelRoute(s *toki.Scanner, table Table) error {
	t := s.Next()
	if t.Token != word {
		return errors.New("need route key")
	}
	key := string(t.Value)
	return table.DelRoute(key)
}

func readModDest(s *toki.Scanner, table Table) error {
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
	for t.Token != toki.EOF {
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

func readModRoute(s *toki.Scanner, table Table) error {
	t := s.Next()
	if t.Token != word {
		return errFmtAddRoute
	}
	key := string(t.Value)

	opts := make(map[string]string)
	for t.Token != toki.EOF {
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
func readDestinations(s *toki.Scanner, table Table, allowMatcher bool, routeKey string) (destinations []*destination.Destination, err error) {
	t := s.Peek()
	for t.Token != toki.EOF {
		for t.Token == sep {
			s.Next()
			t = s.Peek()
		}
		if t.Token == toki.EOF {
			break
		}

		dest, err := readDestination(s, table, allowMatcher, routeKey)
		if err != nil {
			return destinations, err
		}
		destinations = append(destinations, dest)

		t = s.Peek()
	}
	return destinations, nil
}

func readDestination(s *toki.Scanner, table Table, allowMatcher bool, routeKey string) (dest *destination.Destination, err error) {
	var prefix, sub, regex, addr, spoolDir string
	var spool, pickle bool
	flush := 1000
	reconn := 10000
	connBufSize := 30000
	ioBufSize := 2000000
	spoolDir = table.GetSpoolDir()

	spoolBufSize := 10000
	spoolMaxBytesPerFile := int64(200 * 1024 * 1024)
	spoolSyncEvery := int64(10000)
	spoolSyncPeriod := time.Second
	spoolSleep := time.Duration(500) * time.Microsecond
	unspoolSleep := time.Duration(10) * time.Microsecond

	t := s.Next()
	if t.Token != word {
		return nil, errors.New("addr not set for endpoint")
	}
	addr = string(t.Value)

	for t.Token != toki.EOF && t.Token != sep {
		t = s.Next()
		switch t.Token {
		case optPrefix:
			if t = s.Next(); t.Token != word {
				return nil, errFmtAddRoute
			}
			prefix = string(t.Value)
		case optSub:
			if t = s.Next(); t.Token != word {
				return nil, errFmtAddRoute
			}
			sub = string(t.Value)
		case optRegex:
			if t = s.Next(); t.Token != word {
				return nil, errFmtAddRoute
			}
			regex = string(t.Value)
		case optFlush:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			flush, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
		case optReconn:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			reconn, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
		case optPickle:
			if t = s.Next(); t.Token != optTrue && t.Token != optFalse {
				return nil, errFmtAddRoute
			}
			pickle, err = strconv.ParseBool(string(t.Value))
			if err != nil {
				return nil, fmt.Errorf("unrecognized pickle value '%s'", t)
			}
		case optSpool:
			if t = s.Next(); t.Token != optTrue && t.Token != optFalse {
				return nil, errFmtAddRoute
			}
			spool, err = strconv.ParseBool(string(t.Value))
			if err != nil {
				return nil, fmt.Errorf("unrecognized spool value '%s'", t)
			}
		case optConnBufSize:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			connBufSize, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
		case optIoBufSize:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			ioBufSize, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
		case optSpoolBufSize:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			spoolBufSize, err = strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
		case optSpoolMaxBytesPerFile:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			tmp, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
			spoolMaxBytesPerFile = int64(tmp)
		case optSpoolSyncEvery:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			tmp, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
			spoolSyncEvery = int64(tmp)
		case optSpoolSyncPeriod:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			tmp, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
			spoolSyncPeriod = time.Duration(tmp) * time.Millisecond
		case optSpoolSleep:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			tmp, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
			spoolSleep = time.Duration(tmp) * time.Microsecond
		case optUnspoolSleep:
			if t = s.Next(); t.Token != num {
				return nil, errFmtAddRoute
			}
			tmp, err := strconv.Atoi(strings.TrimSpace(string(t.Value)))
			if err != nil {
				return nil, err
			}
			unspoolSleep = time.Duration(tmp) * time.Microsecond
		case toki.EOF:
		case sep:
			break
		default:
			return nil, fmt.Errorf("unrecognized option '%s'", t)
		}
	}

	periodFlush := time.Duration(flush) * time.Millisecond
	periodReConn := time.Duration(reconn) * time.Millisecond
	if !allowMatcher && (prefix != "" || sub != "" || regex != "") {
		return nil, fmt.Errorf("matching options (prefix, sub, and regex) not allowed for this route type")
	}
	routekey := routeKey
	return destination.New(prefix, sub, regex, addr, spoolDir, spool, pickle, periodFlush, periodReConn, connBufSize, ioBufSize, spoolBufSize, spoolMaxBytesPerFile, spoolSyncEvery, spoolSyncPeriod, spoolSleep, unspoolSleep, routekey)
}

func ParseDestinations(destinationConfigs []string, table Table, allowMatcher bool, routeKey string) (destinations []*destination.Destination, err error) {
	s := toki.NewScanner(tokens)
	for _, destinationConfig := range destinationConfigs {
		s.SetInput(destinationConfig)

		dest, err := readDestination(s, table, allowMatcher, routeKey)
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
			return
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
}
