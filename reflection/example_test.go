package reflection_test

import (
	"fmt"
	"time"

	"github.com/Psiphon-Inc/configloader-go/reflection"
	"github.com/Psiphon-Inc/configloader-go/toml"
)

func ExampleGetStructFields_withStruct() {
	const tagName = "conf"

	type S struct {
		F     string `toml:"eff" conf:"optional"`
		Inner struct {
			InnerF int
		}
		Time time.Time // implements encoding.UnmarshalText

		unexported string
	}

	var s S
	structFields := reflection.GetStructFields(s, tagName, toml.Codec)

	for _, sf := range structFields {
		fmt.Println(sf)
	}

	// Output:
	// StructField{
	// 	AliasedKey: [[F eff]]
	// 	Optional: true
	// 	Type: string
	// 	Kind: string
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[Inner]]
	// 	Optional: false
	// 	Type: struct { InnerF int }
	// 	Kind: struct
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[Inner] [InnerF]]
	// 	Optional: false
	// 	Type: int
	// 	Kind: int
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[Time]]
	// 	Optional: false
	// 	Type: time.Time
	// 	Kind: struct
	// 	ExpectedType: string
	// }
}

func ExampleGetStructFields_withMap() {
	const tagName = "conf"

	m := map[string]interface{}{
		"a": "aaa",
		"b": 123,
		"c": time.Now(),
		"d": map[string]interface{}{
			"d1": "d1d1",
		},
		"e": []bool{true, false},
	}

	structFields := reflection.GetStructFields(m, tagName, toml.Codec)

	for _, sf := range structFields {
		fmt.Println(sf)
	}

	// Output:
	// StructField{
	// 	AliasedKey: [[a]]
	// 	Optional: false
	// 	Type: string
	// 	Kind: string
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[b]]
	// 	Optional: false
	// 	Type: int
	// 	Kind: int
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[c]]
	// 	Optional: false
	// 	Type: time.Time
	// 	Kind: struct
	// 	ExpectedType: string
	// }
	// StructField{
	// 	AliasedKey: [[d]]
	// 	Optional: false
	// 	Type: map[string]interface {}
	// 	Kind: map
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[d] [d1]]
	// 	Optional: false
	// 	Type: string
	// 	Kind: string
	// 	ExpectedType:
	// }
	// StructField{
	// 	AliasedKey: [[e]]
	// 	Optional: false
	// 	Type: []bool
	// 	Kind: slice
	// 	ExpectedType:
	// }
}
