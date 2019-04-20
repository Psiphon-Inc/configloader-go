// Package json provides JSON Codec methods for use with psiconfig.
package json

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/Psiphon-Inc/psiphon-go-config/reflection"
	"github.com/pkg/errors"
)

type codecImplmentation struct{}

// Codec is the psiconfig.Codec implementation.
var Codec = codecImplmentation{}

func (codec codecImplmentation) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (codec codecImplmentation) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// Returns true if the struct tag indicates that the field should not be inspected
func (codec codecImplmentation) IsStructFieldIgnored(st reflect.StructTag) bool {
	return st.Get("json") == "-"
}

// Returns empty string if the field has no alias
func (codec codecImplmentation) GetStructFieldAlias(st reflect.StructTag) string {
	if codec.IsStructFieldIgnored(st) {
		return ""
	}

	if typeTag := st.Get("json"); typeTag != "" {
		return strings.Split(typeTag, ",")[0]
	}

	return ""
}

func (codec codecImplmentation) FieldTypesConsistent(check, gold reflection.StructField) (noDeeper bool, err error) {
	if strings.HasPrefix(check.Kind, "float") && (strings.HasPrefix(gold.Kind, "float") || strings.HasPrefix(gold.Kind, "int")) {
		return true, nil
	}

	return false, errors.Errorf("field types inconsistent")
}
