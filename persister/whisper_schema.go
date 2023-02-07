// this is a parser for graphite's storage-schemas.conf
// it supports old and new retention format
// see https://graphite.readthedocs.io/en/0.9.9/config-carbon.html#storage-schemas-conf
// based on https://github.com/grobian/carbonwriter but with some improvements
package persister

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/grafana/carbon-relay-ng/go-whisper"
)

// Schema represents one schema setting
type Schema struct {
	Name         string
	Pattern      *regexp.Regexp
	RetentionStr string
	Retentions   whisper.Retentions
	Priority     int64

	// Optional fields that are only used by Grafana Cloud Graphite and Grafana Enterprise Metrics
	// Documentation https://grafana.com/docs/enterprise-metrics/latest/graphite/schemas/#storage-schemas
	IntervalsStr       string
	RelativeToQueryStr string
}

// WhisperSchemas contains schema settings
type WhisperSchemas []Schema

func (s WhisperSchemas) Len() int           { return len(s) }
func (s WhisperSchemas) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s WhisperSchemas) Less(i, j int) bool { return s[i].Priority >= s[j].Priority }

// Match finds the schema for metric or returns false if none found
func (s WhisperSchemas) Match(metric string) (Schema, bool) {
	for _, schema := range s {
		if schema.Pattern.MatchString(metric) {
			return schema, true
		}
	}
	return Schema{}, false
}

// String() returns the string representation of the whisper schemas.
// It is equivalent, though not identical to the original schemas that were parsed:
// * priorities are derivatives of user input (and include position information), or explicitly 0 if unset in input
// * entries are ordered based on priority
// * the retentions string is provided exactly as the input
// * no comments or extraneous whitespace is omitted
func (s WhisperSchemas) String() string {
	var buf strings.Builder
	for _, schema := range s {
		buf.WriteString(fmt.Sprintf("[%s]\n", schema.Name))
		buf.WriteString(fmt.Sprintf("pattern = %s\n", schema.Pattern.String()))
		buf.WriteString(fmt.Sprintf("retentions = %s\n", schema.RetentionStr))
		buf.WriteString(fmt.Sprintf("priority = %d\n", schema.Priority))
		if schema.IntervalsStr != "" {
			buf.WriteString(fmt.Sprintf("intervals = %s\n", schema.IntervalsStr))
		}
		if schema.RelativeToQueryStr != "" {
			buf.WriteString(fmt.Sprintf("relativeToQuery = %s\n", schema.RelativeToQueryStr))
		}
	}
	return buf.String()
}

// ParseRetentionDefs parses retention definitions into a Retentions structure
func ParseRetentionDefs(retentionDefs string) (whisper.Retentions, error) {
	retentions := make(whisper.Retentions, 0)
	for _, retentionDef := range strings.Split(retentionDefs, ",") {
		retentionDef = strings.TrimSpace(retentionDef)
		parts := strings.Split(retentionDef, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad retentions spec %q", retentionDef)
		}

		// try old format
		val1, err1 := strconv.ParseInt(parts[0], 10, 0)
		val2, err2 := strconv.ParseInt(parts[1], 10, 0)

		if err1 == nil && err2 == nil {
			retention := whisper.NewRetention(int(val1), int(val2))
			retentions = append(retentions, &retention)
			continue
		}

		// try new format
		retention, err := whisper.ParseRetentionDef(retentionDef)
		if err != nil {
			return nil, err
		}
		retentions = append(retentions, retention)
	}
	return retentions, nil
}

// ReadWhisperSchemas reads and parses a storage-schemas.conf file and returns a sorted
// schemas structure
// see https://graphite.readthedocs.io/en/0.9.9/config-carbon.html#storage-schemas-conf
func ReadWhisperSchemas(filename string) (WhisperSchemas, error) {
	config, err := parseIniFile(filename)
	if err != nil {
		return nil, err
	}

	var schemas WhisperSchemas

	for i, section := range config {
		schema := Schema{}
		schema.Name = section["name"]

		if section["pattern"] == "" {
			return nil, fmt.Errorf("[persister] Empty pattern for [%s]", schema.Name)
		}
		schema.Pattern, err = regexp.Compile(section["pattern"])
		if err != nil {
			return nil, fmt.Errorf("[persister] Failed to parse pattern %q for [%s]: %s",
				section["pattern"], schema.Name, err.Error())
		}
		schema.RetentionStr = section["retentions"]
		schema.Retentions, err = ParseRetentionDefs(schema.RetentionStr)

		if err != nil {
			return nil, fmt.Errorf("[persister] Failed to parse retentions %q for [%s]: %s",
				schema.RetentionStr, schema.Name, err.Error())
		}

		p := int64(0)
		if section["priority"] != "" {
			p, err = strconv.ParseInt(section["priority"], 10, 0)
			if err != nil {
				return nil, fmt.Errorf("[persister] Failed to parse priority %q for [%s]: %s", section["priority"], schema.Name, err)
			}
		}
		schema.Priority = int64(p)<<32 - int64(i) // to sort records with same priority by position in file

		schema.IntervalsStr = section["intervals"]
		// The CRNG ini parser is case-insensitive, which is why we look up "relativetoquery" instead of "relativeToQuery".
		// The GrafanaNet/GEM schemas parser is case-sensitive.
		// In future CRNG releases, we might change this parser to be case-sensitive as well.
		schema.RelativeToQueryStr = section["relativetoquery"]

		schemas = append(schemas, schema)
	}

	sort.Sort(schemas)
	return schemas, nil
}
