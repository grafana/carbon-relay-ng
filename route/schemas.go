package route

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lomik/go-carbon/persister"
	"gopkg.in/raintank/schema.v1"
)

func getSchemas(file string) (persister.WhisperSchemas, error) {
	schemas, err := persister.ReadWhisperSchemas(file)
	if err != nil {
		return nil, err
	}
	var defaultFound bool
	for _, schema := range schemas {
		if schema.Pattern.String() == ".*" {
			defaultFound = true
		}
		if len(schema.Retentions) == 0 {
			return nil, fmt.Errorf("retention setting cannot be empty")
		}
	}
	if !defaultFound {
		// good graphite health (not sure what graphite does if there's no .*
		// but we definitely need to always be able to determine which interval to use
		return nil, fmt.Errorf("storage-conf does not have a default '.*' pattern")
	}
	return schemas, nil
}

func parseMetric(buf []byte, schemas persister.WhisperSchemas) (*schema.MetricData, error) {
	errFmt3Fields := "%q: need 3 fields"
	errFmt := "%q: %s"

	msg := strings.TrimSpace(string(buf))

	elements := strings.Fields(msg)
	if len(elements) != 3 {
		return nil, fmt.Errorf(errFmt3Fields, msg)
	}

	name := elements[0]

	val, err := strconv.ParseFloat(elements[1], 64)
	if err != nil {
		return nil, fmt.Errorf(errFmt, msg, err)
	}

	timestamp, err := strconv.ParseUint(elements[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf(errFmt, msg, err)
	}

	s, ok := schemas.Match(name)
	if !ok {
		panic(fmt.Errorf("couldn't find a schema for %q - this is impossible since we asserted there was a default with patt .*", name))
	}

	md := schema.MetricData{
		Name:     name,
		Metric:   name,
		Interval: s.Retentions[0].SecondsPerPoint(),
		Value:    val,
		Unit:     "unknown",
		Time:     int64(timestamp),
		Mtype:    "gauge",
		Tags:     []string{},
		OrgId:    1, // the hosted tsdb service will adjust to the correct OrgId if using a regular key.  orgid 1 is only retained with admin key.
	}
	return &md, nil
}
