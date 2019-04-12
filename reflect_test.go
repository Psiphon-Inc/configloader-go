package psiconfig

import (
	"reflect"
	"testing"
)

func Test_structKeys(t *testing.T) {
	type testStruct struct {
		TestStructExp   int
		testStructUnexp int
	}

	tests := []struct {
		name string
		obj  interface{}
		want []aliasedKey
	}{
		{
			name: "empty struct",
			obj: struct {
			}{},
			want: []aliasedKey{},
		},
		{
			name: "only unexported fields",
			obj: struct {
				a int
				b string
			}{},
			want: []aliasedKey{},
		},
		{
			name: "exported sub-struct with only unexported fields",
			obj: struct {
				S struct {
					a int
					b string
				}
			}{},
			want: []aliasedKey{},
		},
		{
			name: "simple exported fields",
			obj: struct {
				A int
				B string
			}{},
			want: []aliasedKey{
				{
					{"A"},
				},
				{
					{"B"},
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
			want: []aliasedKey{
				{
					{"A"},
				},
				{
					{"B"},
				},
			},
		},
		{
			name: "tagged fields",
			obj: struct {
				A int
				B string
				c bool
				d float32
				E bool    `toml:"elephant"`
				F float32 `toml:"-"`
				G int     `toml:"giraffe,omitempty"`
				H int     `json:"giraffe,omitempty"`
			}{},
			want: []aliasedKey{
				{
					{"A"},
				},
				{
					{"B"},
				},
				{
					{"E", "elephant"},
				},
				{
					{"F"},
				},
				{
					{"G", "giraffe"},
				},
				{
					{"H"},
				},
			},
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
			want: []aliasedKey{
				{
					{"A"},
				},
				{
					{"B", "banana_top"}, {"B1"},
				},
				{
					{"B", "banana_top"}, {"B2", "banana2"},
				},
				{
					{"B", "banana_top"}, {"B3", "banana3_top"}, {"B31"},
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
			want: []aliasedKey{
				{
					{"A"},
				},
				{
					{"B"},
				},
				{
					{"C"},
				},
				{
					{"D"},
				},
				{
					{"E"}, {"TestStructExp"},
				},
				{
					{"F"},
				},
				{
					{"G"}, {"TestStructExp"},
				},
				{
					{"H"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := structKeys(tt.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("structKeys() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
