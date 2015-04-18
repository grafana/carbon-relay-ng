package ogórek

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
)

func bigInt(s string) *big.Int {
	i := new(big.Int)
	i.SetString(s, 10)
	return i
}

func TestMarker(t *testing.T) {
	buf := bytes.Buffer{}
	dec := NewDecoder(&buf)
	dec.mark()
	k, err := dec.marker()
	if err != nil {
		t.Error(err)
	}
	if k != 0 {
		t.Error("no marker found")
	}
}

var graphitePickle1, _ = hex.DecodeString("80025d71017d710228550676616c75657371035d71042847407d90000000000047407f100000000000474080e0000000000047409764000000000047409c40000000000047409d88000000000047409f74000000000047409c74000000000047409cdc00000000004740a10000000000004740a0d800000000004740938800000000004740a00e00000000004740988800000000004e4e655505737461727471054a00d87a5255047374657071064a805101005503656e6471074a00f08f5255046e616d657108552d5a5a5a5a2e55555555555555552e43434343434343432e4d4d4d4d4d4d4d4d2e5858585858585858582e545454710975612e")
var graphitePickle2, _ = hex.DecodeString("286c70300a286470310a53277374617274270a70320a49313338333738323430300a73532773746570270a70330a4938363430300a735327656e64270a70340a49313338353136343830300a73532776616c756573270a70350a286c70360a463437332e300a61463439372e300a61463534302e300a6146313439372e300a6146313830382e300a6146313839302e300a6146323031332e300a6146313832312e300a6146313834372e300a6146323137362e300a6146323135362e300a6146313235302e300a6146323035352e300a6146313537302e300a614e614e617353276e616d65270a70370a5327757365722e6c6f67696e2e617265612e6d616368696e652e6d65747269632e6d696e757465270a70380a73612e")
var graphitePickel3, _ = hex.DecodeString("286c70310a286470320a5327696e74657276616c73270a70330a286c70340a7353276d65747269635f70617468270a70350a5327636172626f6e2e6167656e7473270a70360a73532769734c656166270a70370a4930300a7361286470380a67330a286c70390a7367350a5327636172626f6e2e61676772656761746f72270a7031300a7367370a4930300a736128647031310a67330a286c7031320a7367350a5327636172626f6e2e72656c617973270a7031330a7367370a4930300a73612e")

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{"int", "I5\n.", int64(5)},
		{"float", "F1.23\n.", float64(1.23)},
		{"long", "L12321231232131231231L\n.", bigInt("12321231232131231231")},
		{"None", "N.", None{}},
		{"empty tuple", "(t.", []interface{}{}},
		{"tuple of two ints", "(I1\nI2\ntp0\n.", []interface{}{int64(1), int64(2)}},
		{"nested tuples", "((I1\nI2\ntp0\n(I3\nI4\ntp1\ntp2\n.",
			[]interface{}{[]interface{}{int64(1), int64(2)}, []interface{}{int64(3), int64(4)}}},
		{"tuple with top 1 items from stack", "I0\n\x85.", []interface{}{int64(0)}},
		{"tuple with top 2 items from stack", "I0\nI1\n\x86.", []interface{}{int64(0), int64(1)}},
		{"tuple with top 3 items from stack", "I0\nI1\nI2\n\x87.", []interface{}{int64(0), int64(1), int64(2)}},
		{"empty list", "(lp0\n.", []interface{}{}},
		{"list of numbers", "(lp0\nI1\naI2\naI3\naI4\na.", []interface{}{int64(1), int64(2), int64(3), int64(4)}},
		{"string", "S'abc'\np0\n.", string("abc")},
		{"unicode", "V\\u65e5\\u672c\\u8a9e\np0\n.", string("日本語")},
		{"empty dict", "(dp0\n.", make(map[interface{}]interface{})},
		{"dict with strings", "(dp0\nS'a'\np1\nS'1'\np2\nsS'b'\np3\nS'2'\np4\ns.", map[interface{}]interface{}{"a": "1", "b": "2"}},
		{"GLOBAL and REDUCE opcodes", "cfoo\nbar\nS'bing'\n\x85R.", Call{Callable: Class{Module: "foo", Name: "bar"}, Args: []interface{}{"bing"}}},
		{"LONG_BINPUT opcode", "(lr0000I17\na.", []interface{}{int64(17)}},
		{"graphite message1", string(graphitePickle1), []interface{}{map[interface{}]interface{}{"values": []interface{}{float64(473), float64(497), float64(540), float64(1497), float64(1808), float64(1890), float64(2013), float64(1821), float64(1847), float64(2176), float64(2156), float64(1250), float64(2055), float64(1570), None{}, None{}}, "start": int64(1383782400), "step": int64(86400), "end": int64(1385164800), "name": "ZZZZ.UUUUUUUU.CCCCCCCC.MMMMMMMM.XXXXXXXXX.TTT"}}},
		{"graphite message2", string(graphitePickle2), []interface{}{map[interface{}]interface{}{"values": []interface{}{float64(473), float64(497), float64(540), float64(1497), float64(1808), float64(1890), float64(2013), float64(1821), float64(1847), float64(2176), float64(2156), float64(1250), float64(2055), float64(1570), None{}, None{}}, "start": int64(1383782400), "step": int64(86400), "end": int64(1385164800), "name": "user.login.area.machine.metric.minute"}}},
		{"graphite message3", string(graphitePickel3), []interface{}{map[interface{}]interface{}{"intervals": []interface{}{}, "metric_path": "carbon.agents", "isLeaf": false}, map[interface{}]interface{}{"intervals": []interface{}{}, "metric_path": "carbon.aggregator", "isLeaf": false}, map[interface{}]interface{}{"intervals": []interface{}{}, "metric_path": "carbon.relays", "isLeaf": false}}},
	}
	for _, test := range tests {
		buf := bytes.NewBufferString(test.input)
		dec := NewDecoder(buf)
		v, err := dec.Decode()
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(v, test.expected) {
			t.Errorf("%s: got\n%q\n expected\n%q", test.name, v, test.expected)
		}
	}
}

func TestZeroLengthData(t *testing.T) {
	data := ""
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	if output.BitLen() > 0 {
		t.Fail()
	}
}

func TestValue1(t *testing.T) {
	data := "\xff\x00"
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	target := big.NewInt(255)
	if target.Cmp(output) != 0 {
		t.Fail()
	}
}

func TestValue2(t *testing.T) {
	data := "\xff\x7f"
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	target := big.NewInt(32767)
	if target.Cmp(output) != 0 {
		t.Fail()
	}
}

func TestValue3(t *testing.T) {
	data := "\x00\xff"
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	target := big.NewInt(256)
	target.Neg(target)
	if target.Cmp(output) != 0 {
		t.Logf("\nGot %v\nExpecting %v\n", output, target)
		t.Fail()
	}
}

func TestValue4(t *testing.T) {
	data := "\x00\x80"
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	target := big.NewInt(32768)
	target.Neg(target)
	if target.Cmp(output) != 0 {
		t.Logf("\nGot %v\nExpecting %v\n", output, target)
		t.Fail()
	}
}

func TestValue5(t *testing.T) {
	data := "\x80"
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	target := big.NewInt(128)
	target.Neg(target)
	if target.Cmp(output) != 0 {
		t.Logf("\nGot %v\nExpecting %v\n", output, target)
		t.Fail()
	}
}

func TestValue6(t *testing.T) {
	data := "\x7f"
	output, err := decodeLong(data)
	if err != nil {
		t.Errorf("Error from decodeLong - %v\n", err)
	}
	target := big.NewInt(127)
	if target.Cmp(output) != 0 {
		t.Fail()
	}
}

func BenchmarkSpeed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		data := "\x00\x80"
		_, err := decodeLong(data)
		if err != nil {
			b.Errorf("Error from decodeLong - %v\n", err)
		}
	}
}
