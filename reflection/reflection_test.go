/*
 * BSD 3-Clause License
 * Copyright (c) 2019, Psiphon Inc.
 * All rights reserved.
 */

package reflection

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	confTag   = "conf"
	formatTag = "testtype" // like "toml" or "json"
)

type codecImplmentation struct{}

var codec = codecImplmentation{}

// Returns true if the struct tag indicates that the field should not be inspected
func (codec codecImplmentation) IsStructFieldIgnored(st reflect.StructTag) bool {
	return st.Get(formatTag) == "-"
}

// Returns empty string if the field has no alias
func (codec codecImplmentation) GetStructFieldAlias(st reflect.StructTag) string {
	if codec.IsStructFieldIgnored(st) {
		return ""
	}

	if typeTag := st.Get(formatTag); typeTag != "" {
		return strings.Split(typeTag, ",")[0]
	}

	return ""
}

func Test_getStructFields(t *testing.T) {
	type testStruct struct {
		TestStructExp   int
		testStructUnexp int
	}

	tests := []struct {
		name string
		obj  interface{}
		want []StructField
	}{
		{
			name: "empty struct",
			obj: struct {
			}{},
			want: []StructField{},
		},
		{
			name: "only unexported fields",
			obj: struct {
				a int
				b string
			}{},
			want: []StructField{},
		},
		{
			name: "exported sub-struct with only unexported fields",
			obj: struct {
				S struct {
					a int
					b string
				}
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"S"}},
					Type:         "struct { a int; b string }",
					Kind:         "struct",
					Optional:     false,
					ExpectedType: "",
				},
			},
		},
		{
			name: "simple exported fields",
			obj: struct {
				A int
				B string
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"B"}},
					Type:         "string",
					Kind:         "string",
					Optional:     false,
					ExpectedType: "",
				},
			},
		},
		{
			name: "exported and unexported fields",
			obj: struct {
				A int
				B string
				c bool
				d float32
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"B"}},
					Type:         "string",
					Kind:         "string",
					Optional:     false,
					ExpectedType: "",
				},
			},
		},
		{
			name: "tagged fields",
			obj: struct {
				A int
				B string `conf:"optional" testtype:"bird"`
				c bool
				d float32
				E bool    `testtype:"elephant,omitempty" conf:"optional,string"`
				F float32 `testtype:"-"`
				G int     `conf:",int64" testtype:"giraffe,omitempty"`
				H int     `testtype:"hippo,omitempty" conf:"optional"`
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"B", "bird"}},
					Type:         "string",
					Kind:         "string",
					Optional:     true,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"E", "elephant"}},
					Type:         "bool",
					Kind:         "bool",
					Optional:     true,
					ExpectedType: "string",
				},
				{
					AliasedKey:   AliasedKey{{"G", "giraffe"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "int64",
				},
				{
					AliasedKey:   AliasedKey{{"H", "hippo"}},
					Type:         "int",
					Kind:         "int",
					Optional:     true,
					ExpectedType: "",
				}},
		},
		{
			name: "sub-structs",
			obj: struct {
				A int
				B struct {
					B1 int
					B2 int `testtype:"banana2"`
					B3 struct {
						B31 int
					} `testtype:"banana3_top"`
				} `testtype:"banana_top"`
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"B", "banana_top"}},
					Type:         `struct { B1 int; B2 int "testtype:\"banana2\""; B3 struct { B31 int } "testtype:\"banana3_top\"" }`,
					Kind:         "struct",
					Optional:     false,
					ExpectedType: "",
					Children: []*StructField{
						{
							AliasedKey: AliasedKey{{"B", "banana_top"}, {"B1"}},
						},
						{
							AliasedKey: AliasedKey{{"B", "banana_top"}, {"B2", "banana2"}},
						},
						{
							AliasedKey: AliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}},
						},
					},
				},
				{
					AliasedKey:   AliasedKey{{"B", "banana_top"}, {"B1"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"B", "banana_top"}}},
				},
				{
					AliasedKey:   AliasedKey{{"B", "banana_top"}, {"B2", "banana2"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"B", "banana_top"}}},
				},
				{
					AliasedKey:   AliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}},
					Type:         "struct { B31 int }",
					Kind:         "struct",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"B", "banana_top"}}},
					Children: []*StructField{
						{
							AliasedKey: AliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}, {"B31"}},
						},
					},
				},
				{
					AliasedKey:   AliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}, {"B31"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}}},
				},
			},
		},
		{
			name: "pointers and interfaces, etc.",
			obj: struct {
				A *int
				B interface{}
				C *struct {
					C1 int
				}
				D map[string]int
				E interface{}
				F *testStruct
				G *testStruct
				H []string
			}{E: testStruct{}, G: &testStruct{}},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "*int",
					Kind:         "ptr",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"B"}},
					Type:         "interface {}",
					Kind:         "interface",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"C"}},
					Type:         "*struct { C1 int }",
					Kind:         "ptr",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"D"}},
					Type:         "map[string]int",
					Kind:         "map",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"E"}},
					Type:         "reflection.testStruct",
					Kind:         "struct",
					Optional:     false,
					ExpectedType: "",
					Children: []*StructField{
						{
							AliasedKey: AliasedKey{{"E"}, {"TestStructExp"}},
						},
					},
				},
				{
					AliasedKey:   AliasedKey{{"E"}, {"TestStructExp"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"E"}}},
				},
				{
					AliasedKey:   AliasedKey{{"F"}},
					Type:         "*reflection.testStruct",
					Kind:         "ptr",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"G"}},
					Type:         "reflection.testStruct",
					Kind:         "struct",
					Optional:     false,
					ExpectedType: "",
					Children: []*StructField{
						{
							AliasedKey: AliasedKey{{"G"}, {"TestStructExp"}},
						},
					},
				},
				{
					AliasedKey:   AliasedKey{{"G"}, {"TestStructExp"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"G"}}},
				},
				{
					AliasedKey:   AliasedKey{{"H"}},
					Type:         "[]string",
					Kind:         "slice",
					Optional:     false,
					ExpectedType: ""},
			},
		},
		{
			name: "UnmarshalText handling",
			obj: struct {
				A    int
				Time time.Time `testtype:"the_time" conf:"optional"`
				B    string
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"Time", "the_time"}},
					Type:         "time.Time",
					Kind:         "struct",
					Optional:     true,
					ExpectedType: "string",
				},
				{
					AliasedKey:   AliasedKey{{"B"}},
					Type:         "string",
					Kind:         "string",
					Optional:     false,
					ExpectedType: "",
				},
			},
		},
		{
			name: "pointer to struct",
			obj: &struct {
				A int
				B string
			}{},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"A"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"B"}},
					Type:         "string",
					Kind:         "string",
					Optional:     false,
					ExpectedType: "",
				},
			},
		},
		{
			name: "for map",
			obj: map[string]interface{}{
				"a": "aaa",
				"b": 123,
				"c": time.Now(), // implements encoding.TextUnmarshaler
				"d": map[string]interface{}{
					"d1": "d1d1",
				},
				"e": []bool{true, false},
				"f": nil,
			},
			want: []StructField{
				{
					AliasedKey:   AliasedKey{{"f"}},
					Type:         "interface {}",
					Kind:         "interface",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"a"}},
					Type:         "string",
					Kind:         "string",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"b"}},
					Type:         "int",
					Kind:         "int",
					Optional:     false,
					ExpectedType: "",
				},
				{
					AliasedKey:   AliasedKey{{"c"}},
					Type:         "time.Time",
					Kind:         "struct",
					Optional:     false,
					ExpectedType: "string",
				},
				{
					AliasedKey:   AliasedKey{{"d"}},
					Type:         "map[string]interface {}",
					Kind:         "map",
					Optional:     false,
					ExpectedType: "",
					Children: []*StructField{
						{
							AliasedKey: AliasedKey{{"d"}, {"d1"}},
						},
					},
				},
				{
					AliasedKey:   AliasedKey{{"d"}, {"d1"}},
					Type:         "string",
					Kind:         "string",
					Optional:     false,
					ExpectedType: "",
					Parent:       &StructField{AliasedKey: AliasedKey{{"d"}}},
				},
				{
					AliasedKey:   AliasedKey{{"e"}},
					Type:         "[]bool",
					Kind:         "slice",
					Optional:     false,
					ExpectedType: "",
				},
			},
		},
		{
			name: "empty map",
			obj:  map[string]interface{}{},
			want: []StructField{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStructFields(tt.obj, confTag, codec)

			if len(got) != len(tt.want) {
				t.Fatalf("length of got and want mismatch;\ngot: %#v;\nwant: %#v", got, tt.want)
			}

			// The ordering of got vs tt.want is not guaranteed.
			// Remove from got as we find matches, so we can make sure nothing is missed
			searchGot := make([]*StructField, len(got))
			copy(searchGot, got)
		WantLoop:
			for _, w := range tt.want {
				for i := range searchGot {
					if compareStructFields(w, *searchGot[i]) {
						searchGot = append(searchGot[:i], searchGot[i+1:]...)
						continue WantLoop
					}
				}

				// Failed to find a match
				t.Fatalf("want field unmatched in got: %#v;\ngot %v", w, got)
			}

			if len(searchGot) > 0 {
				t.Fatalf("some got fields not in want: %#v", searchGot)
			}
		})
	}
}

func compareStructFields(got, want StructField) bool {
	if !got.AliasedKey.Equal(want.AliasedKey) {
		return false
	}

	if got.Optional != want.Optional {
		return false
	}

	if got.Type != want.Type {
		return false
	}

	if got.Kind != want.Kind {
		return false
	}

	if got.ExpectedType != want.ExpectedType {
		return false
	}

	if (got.Parent != nil) != (want.Parent != nil) {
		return false
	}

	// For Parent, we only compare the key
	if got.Parent != nil && !got.Parent.AliasedKey.Equal(want.Parent.AliasedKey) {
		return false
	}

	// For Children, we only compare the keys
	if len(got.Children) != len(want.Children) {
		return false
	}

	for i := range got.Children {
		if !got.Children[i].AliasedKey.Equal(want.Children[i].AliasedKey) {
			return false
		}
	}

	return true
}

func TestAliasedKeyElem_Equal(t *testing.T) {
	tests := []struct {
		name string
		ake  AliasedKeyElem
		cmp  AliasedKeyElem
		want bool
	}{
		{
			name: "one alias, identical",
			ake:  AliasedKeyElem{"k1"},
			cmp:  AliasedKeyElem{"k1"},
			want: true,
		},
		{
			name: "one alias, differ by case",
			ake:  AliasedKeyElem{"k1"},
			cmp:  AliasedKeyElem{"K1"},
			want: true,
		},
		{
			name: "multiple aliases",
			ake:  AliasedKeyElem{"k1", "alias"},
			cmp:  AliasedKeyElem{"alias", "K1"},
			want: true,
		},
		{
			name: "multiple aliases, not all matching",
			ake:  AliasedKeyElem{"abc", "alias"},
			cmp:  AliasedKeyElem{"alias", "xyz"},
			want: true,
		},
		{
			name: "unequal lengths",
			ake:  AliasedKeyElem{"abc", "alias"},
			cmp:  AliasedKeyElem{"alias"},
			want: true,
		},
		{
			name: "no match",
			ake:  AliasedKeyElem{"abc", "alias"},
			cmp:  AliasedKeyElem{"xyz", "other"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ake.Equal(tt.cmp); got != tt.want {
				t.Errorf("AliasedKeyElem.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAliasedKey_Equal(t *testing.T) {
	tests := []struct {
		name string
		ak   AliasedKey
		cmp  AliasedKey
		want bool
	}{
		{
			name: "simple",
			ak:   AliasedKey{{"k1"}},
			cmp:  AliasedKey{{"k1"}},
			want: true,
		},
		{
			name: "longer",
			ak:   AliasedKey{{"k1"}, {"k2", "alias2"}, {"k3"}, {"abc", "ALIAS4"}},
			cmp:  AliasedKey{{"k1"}, {"k2"}, {"alias3", "k3"}, {"alias4", "xyz"}},
			want: true,
		},
		{
			name: "no match",
			ak:   AliasedKey{{"k1"}, {"k2", "alias2"}, {"NOMATCH"}, {"abc", "ALIAS4"}},
			cmp:  AliasedKey{{"k1"}, {"k2"}, {"alias3", "k3"}, {"alias4", "xyz"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ak.Equal(tt.cmp); got != tt.want {
				t.Errorf("AliasedKey.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAliasedKey_HasPrefix(t *testing.T) {
	tests := []struct {
		name   string
		ak     AliasedKey
		prefix AliasedKey
		want   bool
	}{
		{
			name:   "identical",
			ak:     AliasedKey{{"k1"}},
			prefix: AliasedKey{{"k1"}},
			want:   true,
		},
		{
			name:   "longer",
			ak:     AliasedKey{{"k1"}, {"k2", "alias2"}, {"k3"}, {"abc", "ALIAS4"}},
			prefix: AliasedKey{{"K1"}, {"K2"}},
			want:   true,
		},
		{
			name:   "no match",
			ak:     AliasedKey{{"k1"}, {"k2", "alias2"}, {"NOMATCH"}, {"abc", "ALIAS4"}},
			prefix: AliasedKey{{"k1"}, {"k2"}, {"alias3", "k3"}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ak.HasPrefix(tt.prefix); got != tt.want {
				t.Errorf("AliasedKey.HasPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
