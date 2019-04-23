// Package toml provides TOML Codec methods for use with configloader.
package toml

import (
	"errors"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Psiphon-Inc/configloader-go/reflection"
)

type codecImplmentation struct{}

// Codec is the configloader.Codec implementation.
var Codec = codecImplmentation{}

func (codec codecImplmentation) Marshal(v interface{}) ([]byte, error) {
	sb := &strings.Builder{}
	enc := toml.NewEncoder(sb)
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}

func (codec codecImplmentation) Unmarshal(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}

// Returns true if the struct tag indicates that the field should not be inspected
func (codec codecImplmentation) IsStructFieldIgnored(st reflect.StructTag) bool {
	return st.Get("toml") == "-"
}

// Returns empty string if the field has no alias
func (codec codecImplmentation) GetStructFieldAlias(st reflect.StructTag) string {
	if codec.IsStructFieldIgnored(st) {
		return ""
	}

	if typeTag := st.Get("toml"); typeTag != "" {
		return strings.Split(typeTag, ",")[0]
	}

	return ""
}

func (codec codecImplmentation) FieldTypesConsistent(check, gold *reflection.StructField) (noDeeper bool, err error) {
	return false, errors.New("toml has no special FieldTypesConsistent checks")
}
