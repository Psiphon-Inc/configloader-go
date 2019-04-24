package configloader

import (
	"encoding"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Psiphon-Inc/configloader-go/json"
	"github.com/Psiphon-Inc/configloader-go/reflection"
	"github.com/Psiphon-Inc/configloader-go/toml"
)

func Test_setMapByKey(t *testing.T) {
	// TODO: Tests that use structFields

	type args struct {
		m            map[string]interface{}
		k            Key
		v            interface{}
		structFields []*reflection.StructField
	}
	type test struct {
		name    string
		args    args
		wantMap map[string]interface{}
		wantErr bool
	}
	tests := make([]test, 0)

	tst := test{}
	tst.name = "simple"
	tst.args.m = map[string]interface{}{}
	tst.args.k = Key{"a", "b", "c"}
	tst.args.v = "val"
	tst.wantMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "val",
			},
		},
	}
	tst.wantErr = false
	tests = append(tests, tst)

	//-----------------------------------------------------------------------

	tst = test{}
	tst.name = "overwrite"
	tst.args.m = map[string]interface{}{
		"a": map[string]interface{}{
			"b": "initial",
		},
	}
	tst.args.k = Key{"a", "b"}
	tst.args.v = "val"
	tst.wantMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": "val",
		},
	}
	tst.wantErr = false
	tests = append(tests, tst)

	//-----------------------------------------------------------------------

	tst = test{}
	tst.name = "nil intermediate"
	tst.args.m = map[string]interface{}{
		"a": nil,
	}
	tst.args.k = Key{"a", "b"}
	tst.args.v = "val"
	tst.wantMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": "val",
		},
	}
	tst.wantErr = false
	tests = append(tests, tst)

	//-----------------------------------------------------------------------

	tst = test{}
	tst.name = "error"
	tst.args.m = map[string]interface{}{
		"a": "initial",
	}
	tst.args.k = Key{"a", "b"}
	tst.args.v = "val"
	tst.wantErr = true
	tests = append(tests, tst)

	//-----------------------------------------------------------------------

	tst = test{}
	tst.name = "matching alias"
	tst.args.m = map[string]interface{}{
		"a": nil,
	}
	tst.args.k = Key{"apple", "b"}
	tst.args.v = "val"
	tst.args.structFields = []*reflection.StructField{
		{AliasedKey: reflection.AliasedKey{{"A", "apple"}}},
		{AliasedKey: reflection.AliasedKey{{"A", "apple"}, {"B", "banana"}}},
	}
	tst.wantMap = map[string]interface{}{
		"a": map[string]interface{}{
			"banana": "val",
		},
	}
	tst.wantErr = false
	tests = append(tests, tst)

	//-----------------------------------------------------------------------

	tst = test{}
	tst.name = "matching alias prefix"
	tst.args.m = map[string]interface{}{
		"a": nil,
	}
	tst.args.k = Key{"apple", "b"}
	tst.args.v = "val"
	tst.args.structFields = []*reflection.StructField{
		{AliasedKey: reflection.AliasedKey{{"A", "apple"}}},
	}
	tst.wantMap = map[string]interface{}{
		"a": map[string]interface{}{
			"b": "val",
		},
	}
	tst.wantErr = false
	tests = append(tests, tst)

	//-----------------------------------------------------------------------

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setMapByKey(tt.args.m, tt.args.k, tt.args.v, tt.args.structFields)
			if err != nil != tt.wantErr {
				t.Fatalf("setMapByKey() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if !reflect.DeepEqual(tt.args.m, tt.wantMap) {
				t.Fatalf("maps don't match\ngot:  %#v\nwant: %#v", tt.args.m, tt.wantMap)
			}
		})
	}
}

type textUnmarshalType struct {
	ExportedString string
	ExportedInt    int
}

func (tut *textUnmarshalType) UnmarshalText(text []byte) error {
	tut.ExportedString = string(text)
	return nil
}

var _ encoding.TextUnmarshaler = &textUnmarshalType{}

func TestLoad(t *testing.T) {
	type stringAlias string
	type boolAlias bool

	type simpleStruct struct {
		A1 string
		B1 int
	}
	type subStruct struct {
		A  string       `conf:"optional"`
		S1 simpleStruct `toml:"sect1" json:"sect1" conf:"optional"`
		S2 simpleStruct `toml:"sect2" json:"sect2" conf:"optional"`
	}
	type tagStruct struct {
		A stringAlias `toml:"eh,omitempty" json:"eh,omitempty" conf:"optional,string"`
		B boolAlias   `toml:"bee" json:"bee" conf:","`
		C string      `toml:"-" json:"-"`
		D float32     `conf:"optional"`
	}
	type advancedTypesStruct struct {
		A *string
		B []int
		C simpleStruct `toml:"cee_three" json:"cee_three"`
		D []simpleStruct
		E time.Time
		F textUnmarshalType
	}
	type comboStruct struct {
		Simple simpleStruct
		Tag    tagStruct
		Adv    advancedTypesStruct
	}
	type hardTypeStruct struct {
		F float64 `conf:",float32"` // BurntSushi/toml will never give us float32, so this should never match
	}
	type subMapStruct struct {
		A string                 `toml:"apple"`
		M map[string]interface{} `toml:"maple"`
	}

	type args struct {
		codec        Codec
		readers      []io.Reader
		readerNames  []string
		envOverrides []EnvOverride
		defaults     []Default
	}
	type test struct {
		name              string
		args              args
		env               map[string]string
		wantConfig        interface{}
		wantProvenances   map[string]string
		wantIsDefineds    []Key
		wantNotIsDefineds []Key
		wantErrIsDefineds []Key
		wantErr           bool
	}
	tests := make([]test, 0)
	var tst test

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "simple toml"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		B1 = 123
		a1 = "aa"
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = simpleStruct{
		A1: "aa",
		B1: 123,
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A1": "0",
		"B1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"a1"},
		{"A1"},
		{"b1"},
		{"B1"},
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{
		{"x"},
		{"X"},
	}
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "simple json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{"B1": 123, "a1": "aa"}`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = simpleStruct{
		A1: "aa",
		B1: 123,
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A1": "0",
		"B1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"a1"},
		{"A1"},
		{"b1"},
		{"B1"},
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{
		{"x"},
		{"X"},
	}
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "simple map"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		B1 = 123
		a1 = "aa"
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = map[string]interface{}{
		"a1": "aa",
		"B1": int64(123),
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"a1": "0",
		"B1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"a1"},
		{"B1"},
	}
	tst.wantNotIsDefineds = []Key{
		{"x"},
		{"X"},
	}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "multi-reader"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		[sect1]
		a1 = "sect1.a1 from first reader"
		[sect2]
		a1 = "sect2.a1 from first reader"
		`,
		`
		[sect1]
		b1 = 21
		[sect2]
		b1 = 22
		`,
		`
		[sect1]
		a1 = "sect1.a1 from third reader"
		[sect2]
		b1 = 32
		`,
	})
	tst.args.readerNames = []string{"first", "second", "third"}
	tst.args.envOverrides = nil
	tst.wantConfig = subStruct{
		S1: simpleStruct{
			A1: "sect1.a1 from third reader",
			B1: 21,
		},
		S2: simpleStruct{
			A1: "sect2.a1 from first reader",
			B1: 32,
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":        "[absent]",
		"sect1.A1": "third",
		"sect1.B1": "second",
		"sect2.A1": "first",
		"sect2.B1": "third",
	}
	tst.wantIsDefineds = []Key{
		{"sect1"}, {"Sect1"},
		{"sect2"}, {"Sect2"},
		{"sect1", "a1"}, {"Sect1", "A1"},
		{"Sect1", "a1"}, {"sect1", "A1"},
		{"sect1", "b1"}, {"Sect1", "B1"},
		{"Sect1", "b1"}, {"sect1", "B1"},
		{"sect2", "a1"}, {"Sect2", "A1"},
		{"Sect2", "a1"}, {"sect2", "A1"},
		{"sect2", "b1"}, {"Sect2", "B1"},
		{"Sect2", "b1"}, {"sect1", "B1"},
	}
	tst.wantNotIsDefineds = []Key{{"A"}}
	tst.wantErrIsDefineds = nil
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "multi-reader json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{
			"sect1": {
				"a1": "sect1.a1 from first reader"
			},
			"sect2": {
				"a1": "sect2.a1 from first reader"
			}
		}`,
		`{
			"sect1": {
				"b1": 21
			},
			"sect2": {
				"b1": 22
			}
		}`,
		`{
			"sect1": {
				"a1": "sect1.a1 from third reader"
			},
			"sect2": {
				"b1": 32
			}
		}`,
	})
	tst.args.readerNames = []string{"first", "second", "third"}
	tst.args.envOverrides = nil
	tst.wantConfig = subStruct{
		S1: simpleStruct{
			A1: "sect1.a1 from third reader",
			B1: 21,
		},
		S2: simpleStruct{
			A1: "sect2.a1 from first reader",
			B1: 32,
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":        "[absent]",
		"sect1.A1": "third",
		"sect1.B1": "second",
		"sect2.A1": "first",
		"sect2.B1": "third",
	}
	tst.wantIsDefineds = []Key{
		{"sect1"}, {"Sect1"},
		{"sect2"}, {"Sect2"},
		{"sect1", "a1"}, {"Sect1", "A1"},
		{"Sect1", "a1"}, {"sect1", "A1"},
		{"sect1", "b1"}, {"Sect1", "B1"},
		{"Sect1", "b1"}, {"sect1", "B1"},
		{"sect2", "a1"}, {"Sect2", "A1"},
		{"Sect2", "a1"}, {"sect2", "A1"},
		{"sect2", "b1"}, {"Sect2", "B1"},
		{"Sect2", "b1"}, {"sect1", "B1"},
	}
	tst.wantNotIsDefineds = []Key{{"A"}}
	tst.wantErrIsDefineds = nil
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "env override"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		[sect2]
		a1 = "s2.a1 from file"
		b1 = 222
		`,
	})
	tst.args.readerNames = []string{"first"}
	tst.args.envOverrides = []EnvOverride{
		{
			EnvVar: "S1A1_FROM_ENV",
			Key:    Key{"S1", "A1"},
			Conv:   nil,
		},
		{
			EnvVar: "S1B1_FROM_ENV",
			Key:    Key{"sect1", "b1"},
			Conv: func(v string) interface{} {
				i, _ := strconv.Atoi(v)
				return i
			},
		},
		{
			EnvVar: "S2A1_FROM_ENV",
			Key:    Key{"sect2", "a1"},
			Conv:   nil,
		},
		{
			EnvVar: "WILL_BE_UNUSED",
			Key:    Key{"S2", "B1"},
			Conv:   nil,
		},
	}
	tst.env = map[string]string{
		"S1A1_FROM_ENV": "S1A1 from env",
		"S1B1_FROM_ENV": "333333",
		"S2A1_FROM_ENV": "S2A1 from env",
	}
	tst.wantConfig = subStruct{
		S1: simpleStruct{
			A1: "S1A1 from env",
			B1: 333333,
		},
		S2: simpleStruct{
			A1: "S2A1 from env",
			B1: 222,
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":        "[absent]",
		"sect1.A1": "$S1A1_FROM_ENV",
		"sect1.B1": "$S1B1_FROM_ENV",
		"sect2.A1": "$S2A1_FROM_ENV",
		"sect2.B1": "first",
	}
	tst.wantIsDefineds = nil
	tst.wantNotIsDefineds = nil
	tst.wantErrIsDefineds = nil
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "env override map"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		[sect1]
		a1 = "s1.a1 from file"
		b1 = 111
		[sect2]
		a1 = "s2.a1 from file"
		b1 = 222
		`,
	})
	tst.args.readerNames = []string{"first"}
	tst.args.envOverrides = []EnvOverride{
		{
			EnvVar: "S1B1_FROM_ENV",
			Key:    Key{"sect1", "b1"},
			Conv: func(v string) interface{} {
				i, _ := strconv.ParseInt(v, 10, 64)
				return i
			},
		},
		{
			EnvVar: "S2A1_FROM_ENV",
			Key:    Key{"sect2", "a1"},
			Conv:   nil,
		},
		{
			EnvVar: "WILL_BE_UNUSED",
			Key:    Key{"sect1", "a1"},
			Conv:   nil,
		},
	}
	tst.env = map[string]string{
		"S1B1_FROM_ENV": "333333",
		"S2A1_FROM_ENV": "from env",
	}
	tst.wantConfig = map[string]interface{}{
		"sect1": map[string]interface{}{
			"a1": "s1.a1 from file",
			"b1": int64(333333),
		},
		"sect2": map[string]interface{}{
			"a1": "from env",
			"b1": int64(222),
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"sect1.a1": "first",
		"sect1.b1": "$S1B1_FROM_ENV",
		"sect2.a1": "$S2A1_FROM_ENV",
		"sect2.b1": "first",
	}
	tst.wantIsDefineds = nil
	tst.wantNotIsDefineds = nil
	tst.wantErrIsDefineds = nil
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: vestigial key"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		a1 = "aa"
		b1 = 22
		c1 = "nope"
		`,
	})
	tst.wantConfig = simpleStruct{}
	tst.args.readerNames = []string{"first"}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "error: vestigial key json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{ "a1": "aa", "b1": 22, "c1": "nope" }`,
	})
	tst.wantConfig = simpleStruct{}
	tst.args.readerNames = []string{"first"}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "error: missing required key"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		a1 = "aa"
		`,
	})
	tst.wantConfig = simpleStruct{}
	tst.args.readerNames = []string{"first"}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------
	tst = test{}
	tst.name = "error: missing required key json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{ "a1": "aa" }`,
	})
	tst.wantConfig = simpleStruct{}
	tst.args.readerNames = []string{"first"}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "tags"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		bee = true
		D = 1.2
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = tagStruct{
		B: true,
		D: 1.2,
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"eh":  "[absent]",
		"bee": "0",
		"D":   "0",
	}
	tst.wantIsDefineds = []Key{
		{"bee"}, {"b"}, {"B"},
	}
	tst.wantNotIsDefineds = []Key{
		{"A"}, {"a"},
	}
	tst.wantErrIsDefineds = []Key{
		{"c"}, {"C"},
	}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "tags json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{
			"bee": true,
			"D": 1.2
		}`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = tagStruct{
		B: true,
		D: 1.2,
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"eh":  "[absent]",
		"bee": "0",
		"D":   "0",
	}
	tst.wantIsDefineds = []Key{
		{"bee"}, {"b"}, {"B"},
	}
	tst.wantNotIsDefineds = []Key{
		{"A"}, {"a"},
	}
	tst.wantErrIsDefineds = []Key{
		{"c"}, {"C"},
	}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "advanced types"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		A = "aaaa"
		B = [1, 1, 2, 3, 5]
		E = "2001-01-01T01:01:01Z"
		F = "my text"
		[cee_three]
		a1 = "a1a1"
		b1 = 321
		[[D]]
		a1 = "1"
		b1 = 1
		[[D]]
		a1 = "2"
		b1 = 2
		[[D]]
		a1 = "3"
		b1 = 3
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	aString := "aaaa"
	eTime, _ := time.Parse(time.RFC3339, "2001-01-01T01:01:01Z")
	tst.wantConfig = advancedTypesStruct{
		A: &aString,
		B: []int{1, 1, 2, 3, 5},
		C: simpleStruct{
			A1: "a1a1",
			B1: 321,
		},
		D: []simpleStruct{
			{A1: "1", B1: 1},
			{A1: "2", B1: 2},
			{A1: "3", B1: 3},
		},
		E: eTime,
		F: textUnmarshalType{
			ExportedString: "my text",
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":            "0",
		"B":            "0",
		"D":            "0",
		"E":            "0",
		"F":            "0",
		"cee_three.A1": "0",
		"cee_three.B1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"C", "A1"},
		{"c", "a1"},
		{"cee_three", "a1"},
		{"F", "ExportedString"}, // Not explicity defined, but implicitly by F's text-unmarshalling
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "advanced types json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{
		"A": "aaaa",
		"B": [1, 1, 2, 3, 5],
		"E": "2001-01-01T01:01:01Z",
		"F": "my text",
		"cee_three": {
			"a1": "a1a1",
			"b1": 321
		},
		"D": [
			{
				"a1": "1",
				"b1": 1
			},
			{
				"a1": "2",
				"b1": 2
			},
			{
				"a1": "3",
				"b1": 3
			}
		]
		}`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	aString = "aaaa"
	eTime, _ = time.Parse(time.RFC3339, "2001-01-01T01:01:01Z")
	tst.wantConfig = advancedTypesStruct{
		A: &aString,
		B: []int{1, 1, 2, 3, 5},
		C: simpleStruct{
			A1: "a1a1",
			B1: 321,
		},
		D: []simpleStruct{
			{A1: "1", B1: 1},
			{A1: "2", B1: 2},
			{A1: "3", B1: 3},
		},
		E: eTime,
		F: textUnmarshalType{
			ExportedString: "my text",
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":            "0",
		"B":            "0",
		"D":            "0",
		"E":            "0",
		"F":            "0",
		"cee_three.A1": "0",
		"cee_three.B1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"C", "A1"},
		{"c", "a1"},
		{"cee_three", "a1"},
		{"F", "ExportedString"}, // Not explicity defined, but implicitly by F's text-unmarshalling
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "advanced types map"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		A = "aaaa"
		B = [1, 1, 2, 3, 5]
		E = "2001-01-01T01:01:01Z"
		F = "my text"
		[cee_three]
		a1 = "a1a1"
		b1 = 321
		[[D]]
		a1 = "1"
		b1 = 1
		[[D]]
		a1 = "2"
		b1 = 2
		[[D]]
		a1 = "3"
		b1 = 3
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	aString = "aaaa"
	tst.wantConfig = map[string]interface{}{
		"A": "aaaa",
		"B": []interface{}{int64(1), int64(1), int64(2), int64(3), int64(5)},
		"cee_three": map[string]interface{}{
			"a1": "a1a1",
			"b1": int64(321),
		},
		"D": []map[string]interface{}{
			{"a1": "1", "b1": int64(1)},
			{"a1": "2", "b1": int64(2)},
			{"a1": "3", "b1": int64(3)},
		},
		"E": "2001-01-01T01:01:01Z",
		"F": "my text",
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":            "0",
		"B":            "0",
		"D":            "0",
		"E":            "0",
		"F":            "0",
		"cee_three.a1": "0",
		"cee_three.b1": "0",
	}
	tst.wantIsDefineds = []Key{
		{"cee_three", "a1"},
		{"D"},
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	// BurntSushi/toml will fill the struct with zero values and give no error
	type structWithString struct {
		F string
	}
	type structWithSub struct {
		Struct structWithString
	}
	tst.name = "error: wrong type for struct"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		Struct = "string is the wrong type"
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = structWithSub{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: wrong type for struct json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		Struct = "string is the wrong type"
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.wantConfig = structWithSub{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: multi-reader name mismatch"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		[sect1]
		a1 = "sect1.a1 from first reader"
		[sect2]
		a1 = "sect2.a1 from first reader"
		`,
		`
		[sect1]
		b1 = 21
		[sect2]
		b1 = 22
		`,
		`
		[sect1]
		a1 = "sect1.a1 from third reader"
		[sect2]
		b1 = 32
		`,
	})
	tst.args.readerNames = []string{"first", "second", "third", "invalid"}
	tst.args.envOverrides = nil
	tst.wantConfig = subStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: bad toml"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		not really TOML
		`,
	})
	tst.args.readerNames = []string{"first"}
	tst.wantConfig = subStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: bad json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		not really JSON
		`,
	})
	tst.args.readerNames = []string{"first"}
	tst.wantConfig = subStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: env override with unfindable key"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		[sect1]
		a1 = "s1.a1 from file"
		b1 = 111
		[sect2]
		a1 = "s2.a1 from file"
		b1 = 222
		`,
	})
	tst.args.readerNames = []string{"first"}
	tst.args.envOverrides = []EnvOverride{
		{
			EnvVar: "S1B1_FROM_ENV",
			Key:    Key{"sect1", "b1"},
			Conv: func(v string) interface{} {
				i, _ := strconv.Atoi(v)
				return i
			},
		},
		{
			EnvVar: "BAD_KEY",
			Key:    Key{"sect1", "nope"},
			Conv:   nil,
		},
	}
	tst.env = map[string]string{
		"S1B1_FROM_ENV": "333333",
		"BAD_KEY":       "erroneous",
	}
	tst.wantConfig = subStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: env override with empty key"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		[sect1]
		a1 = "s1.a1 from file"
		b1 = 111
		[sect2]
		a1 = "s2.a1 from file"
		b1 = 222
		`,
	})
	tst.args.readerNames = []string{"first"}
	tst.args.envOverrides = []EnvOverride{
		{
			EnvVar: "S1B1_FROM_ENV",
			Key:    Key{"sect1", "b1"},
			Conv: func(v string) interface{} {
				i, _ := strconv.Atoi(v)
				return i
			},
		},
		{
			EnvVar: "BAD_KEY",
			Key:    Key{},
			Conv:   nil,
		},
	}
	tst.env = map[string]string{
		"S1B1_FROM_ENV": "333333",
		"BAD_KEY":       "erroneous",
	}
	tst.wantConfig = subStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: explicit type fail"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		F = 1.2
		`,
	})
	tst.wantConfig = hardTypeStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: explicit type fail json"
	tst.args.codec = json.Codec
	tst.args.readers = makeStringReaders([]string{
		`{ "F": 1.2 }`,
	})
	tst.wantConfig = hardTypeStruct{}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "defaults"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		// Note B is absent even though not tagged optional
		`
		D = 1.2
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.args.defaults = []Default{
		{
			Key: Key{"A"}, // struct key, not alias
			Val: "default A",
		},
		{
			Key: Key{"bee"}, // alias
			Val: true,
		},
		{
			Key: Key{"D"},
			Val: float32(2.3),
		},
	}
	tst.wantConfig = tagStruct{
		A: "default A",
		B: true,
		D: 1.2,
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"eh":  "[default]",
		"bee": "[default]",
		// C is an ignored field
		"D": "0",
	}
	tst.wantIsDefineds = []Key{
		{"A"}, {"a"}, {"eh"},
		{"bee"}, {"b"}, {"B"},
		{"d"}, {"D"},
	}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{
		{"c"}, {"C"},
	}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "defaults map"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		D = 1.2
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.args.defaults = []Default{
		{
			Key: Key{"A"},
			Val: "default A",
		},
		{
			Key: Key{"bee"}, // alias
			Val: true,
		},
		{
			Key: Key{"D"},
			Val: float32(2.3),
		},
	}
	tst.wantConfig = map[string]interface{}{
		"A":   "default A",
		"bee": true,
		"D":   1.2,
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":   "[default]",
		"bee": "[default]",
		"D":   "0",
	}
	tst.wantIsDefineds = []Key{
		{"A"},
		{"bee"},
		{"D"},
	}
	tst.wantNotIsDefineds = []Key{
		{"c"}, {"a"},
	}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "defaults with empty section -- struct"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		A = "aaaa"
		[sect1]
		# This is an empty section with examples
		# A1 = "1.a1a1"
		# B1 = 123
		[sect2]
		A1 = "2.a1a1"
		B1 = 321
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.args.defaults = []Default{
		{
			Key: Key{"sect1", "A1"},
			Val: "default A1",
		},
		{
			Key: Key{"sect1", "B1"},
			Val: 789,
		},
	}
	tst.wantConfig = subStruct{
		A: "aaaa",
		S1: simpleStruct{
			A1: "default A1",
			B1: 789,
		},
		S2: simpleStruct{
			A1: "2.a1a1",
			B1: 321,
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":        "0",
		"sect1.A1": "[default]",
		"sect1.B1": "[default]",
		"sect2.A1": "0",
		"sect2.B1": "0",
	}
	tst.wantIsDefineds = []Key{}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "error: bad defaults key"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		// Note B is absent even though not tagged optional
		`
		D = 1.2
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.args.defaults = []Default{
		{
			Key: Key{"A"}, // struct key, not alias
			Val: "default A",
		},
		{
			Key: Key{"bee"}, // alias
			Val: true,
		},
		{
			Key: Key{"D"},
			Val: float32(2.3),
		},
		{
			Key: Key{"Nope"},
			Val: "not in struct",
		},
	}
	tst.wantConfig = tagStruct{
		A: "default A",
		B: true,
		D: 1.2,
	}
	tst.wantErr = true
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "optional section"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		A = "aaaa"
		[sect1]
		A1 = "1.a1a1"
		B1 = 123
		# [sect2]
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.args.defaults = nil
	tst.wantConfig = subStruct{
		A: "aaaa",
		S1: simpleStruct{
			A1: "1.a1a1",
			B1: 123,
		},
		// S2: Zero value
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"A":        "0",
		"sect1.A1": "0",
		"sect1.B1": "0",
		"sect2.A1": "[absent]",
		"sect2.B1": "[absent]",
	}
	tst.wantIsDefineds = []Key{}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	tst = test{}
	tst.name = "struct with sub-map"
	tst.args.codec = toml.Codec
	tst.args.readers = makeStringReaders([]string{
		`
		apple = "aaaa"
		[maple]
		k1 = "v1"
		k2 = 22
		k3 = false
		arr = ["one", "two", "three"]
		[maple.sub]
		subk1 = "subk1value"
		`,
	})
	tst.args.readerNames = nil
	tst.args.envOverrides = nil
	tst.args.defaults = nil
	tst.wantConfig = subMapStruct{
		A: "aaaa",
		M: map[string]interface{}{
			"k1":  "v1",
			"k2":  int64(22),
			"k3":  false,
			"arr": []interface{}{"one", "two", "three"},
			"sub": map[string]interface{}{
				"subk1": "subk1value",
			},
		},
	}
	tst.wantErr = false
	tst.wantProvenances = map[string]string{
		"apple":           "0",
		"maple.k1":        "0",
		"maple.k2":        "0",
		"maple.k3":        "0",
		"maple.arr":       "0",
		"maple.sub.subk1": "0",
	}
	tst.wantIsDefineds = []Key{}
	tst.wantNotIsDefineds = []Key{}
	tst.wantErrIsDefineds = []Key{}
	tests = append(tests, tst)

	//----------------------------------------------------------------------

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for envKey, envVal := range tt.env {
				os.Setenv(envKey, envVal)
			}

			// Create an instance of the result based on the type of wantConfig
			resultPtr := reflect.New(reflect.TypeOf(tt.wantConfig)).Interface()

			gotMD, err := Load(tt.args.codec, tt.args.readers, tt.args.readerNames, tt.args.envOverrides, tt.args.defaults, resultPtr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v; wantErr: %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			resultComparator := reflect.ValueOf(resultPtr).Elem().Interface()
			if !reflect.DeepEqual(resultComparator, tt.wantConfig) {
				/*
					// Code for debugging map mismatches
					r := resultComparator.(map[string]interface{})
					w := tt.wantConfig.(map[string]interface{})
					if len(r) != len(w) {
						t.Fatalf("length bad")
					}
					for k := range r {
						if reflect.TypeOf(r[k]) != reflect.TypeOf(w[k]) {
							t.Fatalf("types bad for %v: %v vs %v", k, reflect.TypeOf(r[k]), reflect.TypeOf(w[k]))
						}
					}
					rstruct := reflection.GetStructFields(r, TagName, tt.args.codec)
					wstruct := reflection.GetStructFields(w, TagName, tt.args.codec)
					t.Fatalf("structfields\ngot  %+v\nwant %+v", rstruct, wstruct)
				*/

				t.Fatalf("result did not match;\ngot  %#v\nwant %#v\nmd: %+v", resultComparator, tt.wantConfig, gotMD)
			}

			compareProvenances(t, gotMD.Provenances, tt.wantProvenances)

			for _, k := range tt.wantIsDefineds {
				def, err := gotMD.IsDefined(k...)
				if err != nil {
					t.Fatalf("IsDefined should not get error: %v;\nkey: %#v;\nmetadata: %#v", err, k, gotMD)
				}
				if !def {
					t.Fatalf("key should be defined: %#v; metadata: %#v", k, gotMD)
				}
			}

			for _, k := range tt.wantNotIsDefineds {
				def, err := gotMD.IsDefined(k...)
				if err != nil {
					t.Fatalf("IsDefined should not get error; key: %#v; metadata: %#v", k, gotMD)
				}
				if def {
					t.Fatalf("key should be undefined: %#v; metadata: %#v", k, gotMD)
				}
			}

			for _, k := range tt.wantErrIsDefineds {
				_, err := gotMD.IsDefined(k...)
				if err == nil {
					t.Fatalf("IsDefined should get error; key: %#v; metadata: %#v", k, gotMD)
				}
			}
		})
	}
}

func Test_aliasedKeysMatch(t *testing.T) {
	type args struct {
		ak1 reflection.AliasedKey
		ak2 reflection.AliasedKey
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "simple",
			args: args{
				ak1: reflection.AliasedKey{
					{"a"},
				},
				ak2: reflection.AliasedKey{
					{"a"},
				},
			},
			want: true,
		},
		{
			name: "case insensitive",
			args: args{
				ak1: reflection.AliasedKey{
					{"a"},
				},
				ak2: reflection.AliasedKey{
					{"A"},
				},
			},
			want: true,
		},
		{
			name: "aliases",
			args: args{
				ak1: reflection.AliasedKey{
					{"a", "apple"},
				},
				ak2: reflection.AliasedKey{
					{"apple"},
				},
			},
			want: true,
		},
		{
			name: "complex",
			args: args{
				ak1: reflection.AliasedKey{
					{"a", "apple"},
					{"b", "bee"},
					{"C", "carrot"},
				},
				ak2: reflection.AliasedKey{
					{"apple"},
					{"B"},
					{"carrot", "chocolate"},
				},
			},
			want: true,
		},
		{
			name: "no match",
			args: args{
				ak1: reflection.AliasedKey{
					{"a", "apple"},
					{"b", "bee"},
					{"C", "carrot"},
				},
				ak2: reflection.AliasedKey{
					{"apple"},
					{"B"},
					{"nomatch"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.args.ak1.Equal(tt.args.ak2); got != tt.want {
				t.Errorf("aliasedKeysMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findStructField(t *testing.T) {
	type args struct {
		fields    []*reflection.StructField
		targetKey reflection.AliasedKey
	}
	tests := []struct {
		name      string
		args      args
		wantMatch bool
	}{
		{
			name: "simple",
			args: args{
				targetKey: reflection.AliasedKey{
					{"a"},
				},
				fields: []*reflection.StructField{
					{
						AliasedKey: reflection.AliasedKey{
							{"a"},
						},
					},
				},
			},
			wantMatch: true,
		},
		{
			name: "aliases",
			args: args{
				targetKey: reflection.AliasedKey{
					{"b"},
					{"bee_two"},
				},
				fields: []*reflection.StructField{
					{
						AliasedKey: reflection.AliasedKey{
							{"a"},
						},
					},
					{
						AliasedKey: reflection.AliasedKey{
							{"b", "bee"},
						},
					},
					{
						AliasedKey: reflection.AliasedKey{
							{"b", "bee"},
							{"b2", "bee_two"},
						},
					},
					{
						AliasedKey: reflection.AliasedKey{
							{"c", "cee"},
							{"c2", "cee_two"},
						},
					},
				},
			},
			wantMatch: true,
		},
		{
			name: "no match",
			args: args{
				targetKey: reflection.AliasedKey{
					{"b"},
					{"nomatch"},
				},
				fields: []*reflection.StructField{
					{
						AliasedKey: reflection.AliasedKey{
							{"a"},
						},
					},
					{
						AliasedKey: reflection.AliasedKey{
							{"b", "bee"},
						},
					},
					{
						AliasedKey: reflection.AliasedKey{
							{"b", "bee"},
							{"b2", "bee_two"},
						},
					},
					{
						AliasedKey: reflection.AliasedKey{
							{"c", "cee"},
							{"c2", "cee_two"},
						},
					},
				},
			},
			wantMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStructField, ok := findStructField(tt.args.fields, tt.args.targetKey)
			if ok != tt.wantMatch {
				t.Fatalf("findStructField() ok = %v, want %v", ok, tt.wantMatch)
			}

			if !ok {
				return
			}

			if !gotStructField.AliasedKey.Equal(tt.args.targetKey) {
				t.Fatalf("gotStructField doesn't actually match target: got %+v; want %+v", gotStructField.AliasedKey, tt.args.targetKey)
			}
		})
	}
}

func TestLoad_BadArgs(t *testing.T) {
	type Struct struct {
		A string
	}

	var notPtr Struct

	_, gotErr := Load(toml.Codec, makeStringReaders([]string{"\na=1\n"}), nil, nil, nil, notPtr)

	// Didn't pass in &notPtr, so should get error
	if gotErr == nil {
		t.Fatal("Should have got error for not passing in reference")
	}

	var nilPtr *Struct

	_, gotErr = Load(toml.Codec, makeStringReaders([]string{"\na=1\n"}), nil, nil, nil, nilPtr)

	// Passing in nil, so should get error
	if gotErr == nil {
		t.Fatal("Should have got error for passing in nil")
	}
}

func TestLoad_Special(t *testing.T) {
	//
	// Test: Pass in a non-empty map
	//
	{
		result := map[string]interface{}{
			"a": map[string]interface{}{
				"b": "initial",
				"c": "initial",
			},
		}
		readers := makeStringReaders([]string{
			`
		[a]
		b = "from config"
		d = "from config"
		`,
		})
		md, err := Load(toml.Codec, readers, nil, nil, nil, &result)
		if err != nil {
			t.Fatalf("Got error for non-empty map: %v", err)
		}
		want := map[string]interface{}{
			"a": map[string]interface{}{
				"b": "from config",
				"c": "initial",
				"d": "from config",
			},
		}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("Non-empty map result didn't match want;\ngot:  %#v\nwant: %#v", result, want)
		}
		compareProvenances(t, md.Provenances, map[string]string{
			"a.b": "0",
			//"a.c": "[absent]", // Doesn't end up in provenances at all
			"a.d": "0",
		})
		if !reflect.DeepEqual(md.ConfigMap, want) {
			t.Fatalf("md.ConfigMap didn't match;\ngot:  %#v\nwant: %#v", md.ConfigMap, want)
		}
	}

	//
	// Test: Pass in a non-zero struct
	//
	{
		type strct struct {
			A string `conf:"optional"` // will be pre-filled
			B string
		}

		result := strct{
			A: "pre-filled",
			B: "pre-filled",
		}
		readers := makeStringReaders([]string{
			`
			b = "from config"
			`,
		})
		md, err := Load(toml.Codec, readers, nil, nil, nil, &result)
		if err != nil {
			t.Fatalf("Got error for non-zero struct: %v", err)
		}
		want := strct{
			A: "pre-filled",
			B: "from config",
		}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("Non-zero struct result didn't match want;\ngot:  %#v\nwant: %#v", result, want)
		}
		compareProvenances(t, md.Provenances, map[string]string{
			"A": "[absent]",
			"B": "0",
		})
		wantConfigMap := map[string]interface{}{
			"A": "pre-filled",
			"B": "from config",
		}
		if !reflect.DeepEqual(md.ConfigMap, wantConfigMap) {
			t.Fatalf("md.ConfigMap didn't match;\ngot:  %#v\nwant: %#v", md.ConfigMap, wantConfigMap)
		}
	}
}

func TestKey_String(t *testing.T) {
	tests := []struct {
		name string
		k    Key
		want string
	}{
		{
			name: "simple",
			k:    Key{"a", "b"},
			want: "a.b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.k.String(); got != tt.want {
				t.Errorf("Key.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvenance_String(t *testing.T) {
	tests := []struct {
		name string
		prov Provenance
		want string
	}{
		{
			name: "simple",
			prov: Provenance{
				aliasedKey: reflection.AliasedKey{
					{"a"}, {"b"},
				},
				Key: Key{"a", "b"},
				Src: "thesrc",
			},
			want: "'a.b':'thesrc'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.prov.String(); got != tt.want {
				t.Errorf("Provenance.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvenances_String(t *testing.T) {
	tests := []struct {
		name  string
		provs Provenances
		want  string
	}{
		{
			name: "simple",
			provs: Provenances{
				{
					aliasedKey: reflection.AliasedKey{
						{"a"}, {"b"},
					},
					Key: Key{"a", "b"},
					Src: "thesrc",
				},
				{
					aliasedKey: reflection.AliasedKey{
						{"c"}, {"d"},
					},
					Key: Key{"c", "d"},
					Src: "thesrc",
				},
			},
			want: "{ 'a.b':'thesrc'; 'c.d':'thesrc' }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provs.String(); got != tt.want {
				t.Errorf("Provenances.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makeStringReaders(ss []string) []io.Reader {
	res := make([]io.Reader, len(ss))
	for i := range ss {
		res[i] = strings.NewReader(ss[i])
	}
	return res
}

func compareProvenances(t *testing.T, got Provenances, want map[string]string) {
	if len(got) != len(want) {
		t.Fatalf("Provenances: len(got) != len(want);\ngot:  %s\nwant: %s", got, want)
	}
	for key, src := range want {
		found := false
		for _, prov := range got {
			if prov.Key.String() == key && prov.Src == src {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Provenance mismatch; failed to find %s:%s\ngot:  %#v\nwant: %s", key, src, got, want)
		}
	}
}
