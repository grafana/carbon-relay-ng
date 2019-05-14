package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkPlainValidatePacketsStrict(B *testing.B) {
	metric := []byte("abcde.test.test.test 21300.00 12351123")
	h := NewPlain(true, false)
	for i := 0; i < B.N; i++ {
		h.validateKey(metric)
	}
}
func BenchmarkPlainValidatePacketsLoose(B *testing.B) {
	metric := []byte("abcde.test.test.test 21300.00 12351123")
	h := NewPlain(false, false)
	for i := 0; i < B.N; i++ {
		h.validateKey(metric)
	}
}

func TestValidationInvalid(t *testing.T) {
	h := NewPlain(false, false)
	metrics := map[string][]byte{
		"incorrectFields": []byte("incorrect fields 21300.00 12351123"),
		"stringValue":     []byte("incorrect_value two 12351123"),
		"stringTime":      []byte("incorrect_time 1.0 two"),
	}
	for test, metric := range metrics {
		t.Run(test, func(t *testing.T) {
			_, err := h.load(metric)
			assert.Error(t, err)
			_, err = h.loadFaster(metric)
			assert.Error(t, err)
		})
	}
}
func TestValidationValid(t *testing.T) {
	h := NewPlain(false, false)
	metrics := map[string][]byte{
		"normal":       []byte("test.metric 10.00 1000"),
		"spaces":       []byte("   test.metric   10.00  1000"),
		"dotted":       []byte(".test.metric 10.00 1000"),
		"dotted_space": []byte("    .test.metric     10.00      1000"),
	}
	refD := Datapoint{Name: "test.metric", Value: 10.0, Timestamp: 1000}
	for test, metric := range metrics {
		t.Run(test, func(t *testing.T) {
			d, err := h.load(metric)
			assert.NoError(t, err)
			assert.Equal(t, refD, d)

			d, err = h.loadFaster(metric)
			assert.NoError(t, err)
			assert.Equal(t, refD, d)
		})
	}
}

func BenchmarkPlainLoadPackets(B *testing.B) {
	metric := []byte("abcde.test.test.test.test.test.test.test.test.test.ineed.a.one.hundred.byte.metric 21300.00 12351123")
	h := NewPlain(false, false)
	B.Run("Normal", func(B *testing.B) {
		for i := 0; i < B.N; i++ {
			h.load(metric)
		}
	})
	B.Run("AllocFree", func(B *testing.B) {
		for i := 0; i < B.N; i++ {
			h.loadFaster(metric)
		}
	})
}
