package psiconfig

import (
	"reflect"
	"testing"
	"time"
)

func Test_getStructFields(t *testing.T) {
	type testStruct struct {
		TestStructExp   int
		testStructUnexp int
	}

	tests := []struct {
		name string
		obj  interface{}
		want []structField
	}{
		{
			name: "empty struct",
			obj: struct {
			}{},
			want: []structField{},
		},
		{
			name: "only unexported fields",
			obj: struct {
				a int
				b string
			}{},
			want: []structField{},
		},
		{
			name: "exported sub-struct with only unexported fields",
			obj: struct {
				S struct {
					a int
					b string
				}
			}{},
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"S"}},
					typ:          "struct { a int; b string }",
					kind:         "struct",
					optional:     false,
					expectedType: "",
				},
			},
		},
		{
			name: "simple exported fields",
			obj: struct {
				A int
				B string
			}{},
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B"}},
					typ:          "string",
					kind:         "string",
					optional:     false,
					expectedType: "",
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
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B"}},
					typ:          "string",
					kind:         "string",
					optional:     false,
					expectedType: "",
				},
			},
		},
		{
			name: "tagged fields",
			obj: struct {
				A int
				B string `psiconfig:"optional" toml:"bird"`
				c bool
				d float32
				E bool    `toml:"elephant,omitempty" psiconfig:"optional,string"`
				F float32 `toml:"-"`
				G int     `psiconfig:",int64" json:"giraffe,omitempty"`
				H int     `json:"hippo,omitempty" psiconfig:"optional"`
			}{},
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B", "bird"}},
					typ:          "string",
					kind:         "string",
					optional:     true,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"E", "elephant"}},
					typ:          "bool",
					kind:         "bool",
					optional:     true,
					expectedType: "string",
				},
				{
					aliasedKey:   aliasedKey{{"G", "giraffe"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "int64",
				},
				{
					aliasedKey:   aliasedKey{{"H", "hippo"}},
					typ:          "int",
					kind:         "int",
					optional:     true,
					expectedType: "",
				}},
		},
		{
			name: "sub-structs",
			obj: struct {
				A int
				B struct {
					B1 int
					B2 int `toml:"banana2"`
					B3 struct {
						B31 int
					} `toml:"banana3_top"`
				} `toml:"banana_top"`
			}{},
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B", "banana_top"}},
					typ:          `struct { B1 int; B2 int "toml:\"banana2\""; B3 struct { B31 int } "toml:\"banana3_top\"" }`,
					kind:         "struct",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B", "banana_top"}, {"B1"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B", "banana_top"}, {"B2", "banana2"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}},
					typ:          "struct { B31 int }",
					kind:         "struct",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B", "banana_top"}, {"B3", "banana3_top"}, {"B31"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
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
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "*int",
					kind:         "ptr",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B"}},
					typ:          "interface {}",
					kind:         "interface",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"C"}},
					typ:          "*struct { C1 int }",
					kind:         "ptr",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"D"}},
					typ:          "map[string]int",
					kind:         "map",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"E"}},
					typ:          "psiconfig.testStruct",
					kind:         "struct",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"E"}, {"TestStructExp"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"F"}},
					typ:          "*psiconfig.testStruct",
					kind:         "ptr",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"G"}},
					typ:          "psiconfig.testStruct",
					kind:         "struct",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"G"}, {"TestStructExp"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"H"}},
					typ:          "[]string",
					kind:         "slice",
					optional:     false,
					expectedType: ""},
			},
		},
		{
			name: "UnmarshalText handling",
			obj: struct {
				A    int
				Time time.Time `json:"the_time" psiconfig:"optional"`
				B    string
			}{},
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"Time", "the_time"}},
					typ:          "time.Time",
					kind:         "struct",
					optional:     true,
					expectedType: "string",
				},
				{
					aliasedKey:   aliasedKey{{"B"}},
					typ:          "string",
					kind:         "string",
					optional:     false,
					expectedType: "",
				},
			},
		},
		{
			name: "pointer to struct",
			obj: &struct {
				A int
				B string
			}{},
			want: []structField{
				{
					aliasedKey:   aliasedKey{{"A"}},
					typ:          "int",
					kind:         "int",
					optional:     false,
					expectedType: "",
				},
				{
					aliasedKey:   aliasedKey{{"B"}},
					typ:          "string",
					kind:         "string",
					optional:     false,
					expectedType: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStructFields(tt.obj)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("getStructFields() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
