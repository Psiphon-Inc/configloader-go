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
	// 	Parent: nil
	// 	Children: {}
	// }
	// StructField{
	// 	AliasedKey: [[Inner]]
	// 	Optional: false
	// 	Type: struct { InnerF int }
	// 	Kind: struct
	// 	ExpectedType:
	// 	Parent: nil
	// 	Children: {
	// 		[[Inner] [InnerF]]
	// 	}
	// }
	// StructField{
	// 	AliasedKey: [[Inner] [InnerF]]
	// 	Optional: false
	// 	Type: int
	// 	Kind: int
	// 	ExpectedType:
	// 	Parent: [[Inner]]
	// 	Children: {}
	// }
	// StructField{
	// 	AliasedKey: [[Time]]
	// 	Optional: false
	// 	Type: time.Time
	// 	Kind: struct
	// 	ExpectedType: string
	// 	Parent: nil
	// 	Children: {}
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
	// 	Parent: nil
	// 	Children: {}
	// }
	// StructField{
	// 	AliasedKey: [[b]]
	// 	Optional: false
	// 	Type: int
	// 	Kind: int
	// 	ExpectedType:
	// 	Parent: nil
	// 	Children: {}
	// }
	// StructField{
	// 	AliasedKey: [[c]]
	// 	Optional: false
	// 	Type: time.Time
	// 	Kind: struct
	// 	ExpectedType: string
	// 	Parent: nil
	// 	Children: {}
	// }
	// StructField{
	// 	AliasedKey: [[d]]
	// 	Optional: false
	// 	Type: map[string]interface {}
	// 	Kind: map
	// 	ExpectedType:
	// 	Parent: nil
	// 	Children: {
	// 		[[d] [d1]]
	// 	}
	// }
	// StructField{
	// 	AliasedKey: [[d] [d1]]
	// 	Optional: false
	// 	Type: string
	// 	Kind: string
	// 	ExpectedType:
	// 	Parent: [[d]]
	// 	Children: {}
	// }
	// StructField{
	// 	AliasedKey: [[e]]
	// 	Optional: false
	// 	Type: []bool
	// 	Kind: slice
	// 	ExpectedType:
	// 	Parent: nil
	// 	Children: {}
	// }
}
