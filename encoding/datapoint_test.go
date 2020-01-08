package encoding

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFullName(t *testing.T) {
	tags := Tags{"b": "bbb", "p": "ppp", "a": "aaa"}
	dp := Datapoint{Name: "test.metric", Tags: tags}
	assert.Equal(t, "test.metric;a=aaa;b=bbb;p=ppp", dp.FullName())

}
