package route

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/lomik/go-carbon/persister"
	"gopkg.in/raintank/schema.v1"
)

func getMatchEverythingSchemas() persister.WhisperSchemas {
	schema := persister.WhisperSchemas{
		persister.Schema{
			Name:         "everything",
			RetentionStr: "10:8640",
			Priority:     10,
		},
	}
	schema[0].Retentions, _ = persister.ParseRetentionDefs(schema[0].RetentionStr)
	schema[0].Pattern, _ = regexp.Compile(".*")
	return schema
}

func TestParseMetricWithTags(t *testing.T) {
	schemas := getMatchEverythingSchemas()
	tags := []string{"tag2=value2", "tag1=value1"}
	time := int64(200)
	value := float64(100)
	name := "a.b.c"
	nameWithTags := fmt.Sprintf("%s;%s", name, strings.Join(tags, ";"))
	line := []byte(fmt.Sprintf("%s %f %d", nameWithTags, value, time))
	md, _ := parseMetric(line, schemas, 1)
	sort.Strings(tags)
	expectedMd := &schema.MetricData{
		Name:     name,
		Metric:   name,
		Interval: 10,
		Value:    value,
		Unit:     "unknown",
		Time:     time,
		Mtype:    "gauge",
		Tags:     tags,
		OrgId:    1,
	}
	if !reflect.DeepEqual(md, expectedMd) {
		t.Fatalf("Returned MetricData is not as expected:\nGot:\n%+v\nExpected:\n%+v\n", md, expectedMd)
	}
}

func TestParseMetricWithoutTags(t *testing.T) {
	schemas := getMatchEverythingSchemas()
	time := int64(200)
	value := float64(100)
	name := "a.b.c"
	line := []byte(fmt.Sprintf("%s %f %d", name, value, time))
	md, _ := parseMetric(line, schemas, 1)
	expectedMd := &schema.MetricData{
		Name:     name,
		Metric:   name,
		Interval: 10,
		Value:    value,
		Unit:     "unknown",
		Time:     time,
		Mtype:    "gauge",
		Tags:     []string{},
		OrgId:    1,
	}
	if !reflect.DeepEqual(md, expectedMd) {
		t.Fatalf("Returned MetricData is not as expected:\nGot:\n%+v\nExpected:\n%+v\n", md, expectedMd)
	}
}
