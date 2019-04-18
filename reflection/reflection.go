package reflection

import (
	"encoding"
	"reflect"
	"strings"
)

type Codec interface {
	// Returns true if the struct tag indicates that the field is not unmarshaled (like `json:"-"`)
	IsStructFieldIgnored(st reflect.StructTag) bool

	// Returns name of the field in the config file (like `json:"new_name"`)
	// or empty string if the field has no alias
	GetStructFieldAlias(st reflect.StructTag) string
}

// One component of an aliased key. Contains equivalent aliases for this component.
type AliasedKeyElem []string

// A key that can be written multiple ways, all of which should match. Each element may have aliases.
type AliasedKey []AliasedKeyElem

// Information about field in a struct
type StructField struct {
	AliasedKey   AliasedKey
	Type         string
	Kind         string
	Optional     bool
	ExpectedType string
}

type decoder struct {
	tagName string
	codec   Codec
}

/*
GetStructFields returns the Exported fields found in obj.
The returned slice is guaranteed to have branches before leaves.
tagName is the string used to flag "optional" (and set explicit expected type).

Example with struct:
	type S struct {
		F     string `toml:"eff" conf:"optional"`
		Inner struct {
			InnerF int
		}
		Time time.Time // implements encoding.UnmarshalText

		unexported string
	}

	var s S
	fmt.Printf("%+v\n", GetStructFields(s))
Result:
	[{
		aliasedKey: [
			[F eff]
		] typ: string kind: string optional: true expectedType:
	} {
		aliasedKey: [
			[Inner]
		] typ: struct {
			InnerF int
		}
		kind: struct optional: false expectedType:
	} {
		aliasedKey: [
			[Inner][InnerF]
		] typ: int kind: int optional: false expectedType:
	} {
		aliasedKey: [
			[Time]
		] typ: time.Time kind: struct optional: false expectedType: string
	}]

Example with map:
	m := map[string]interface{}{
	    "a": "aaa",
	    "b": 123,
	    "c": time.Now(),
	    "d": map[string]interface{}{
	        "d1": "d1d1",
	    },
	    "e": []bool{true, false},
	}
	fmt.Printf("%+v\n", GetStructFields(m))
Result:
	[{
		aliasedKey: [
			[a]
		] typ: string kind: string optional: false expectedType:
	} {
		aliasedKey: [
			[b]
		] typ: int kind: int optional: false expectedType:
	} {
		aliasedKey: [
			[c]
		] typ: time.Time kind: struct optional: false expectedType: string
	} {
		aliasedKey: [
			[d]
		] typ: map[string] interface {}
		kind: map optional: false expectedType:
	} {
		aliasedKey: [
			[d][d1]
		] typ: string kind: string optional: false expectedType:
	} {
		aliasedKey: [
			[e]
		] typ: [] bool kind: slice optional: false expectedType:
	}]
*/
func GetStructFields(obj interface{}, tagName string, codec Codec) []StructField {
	d := decoder{tagName, codec}
	return d.getStructFieldsRecursive(reflect.ValueOf(obj), StructField{})
}

// Recursion helper for GetStructFields.
func (d decoder) getStructFieldsRecursive(structValue reflect.Value, currField StructField) []StructField {
	switch structValue.Kind() {
	case reflect.Ptr:
		// Unwrap and recurse
		structValue = structValue.Elem()
		if structValue.IsValid() {
			return d.getStructFieldsRecursive(structValue, currField)
		}

	// If it is a struct we walk each field
	case reflect.Struct:
		structFields := make([]StructField, 0)
		for i := 0; i < structValue.NumField(); i++ {
			field := structValue.Field(i)
			fieldType := structValue.Type().Field(i)
			if fieldType.PkgPath != "" {
				// unexported; see https://golang.org/pkg/reflect/#StructField
				continue
			}

			thisField, recurseValue := d.makeField(
				currField.AliasedKey,
				fieldType.Name,
				&fieldType.Tag,
				field)
			if thisField == nil {
				continue
			}

			structFields = append(structFields, *thisField)

			if recurseValue != nil {
				// Recurse into the field
				structFields = append(structFields, d.getStructFieldsRecursive(*recurseValue, *thisField)...)
			}
		}
		return structFields

	// Recurse into maps
	case reflect.Map:
		mapFields := make([]StructField, 0)
		for _, key := range structValue.MapKeys() {
			fieldValue := structValue.MapIndex(key)

			thisField, recurseValue := d.makeField(
				currField.AliasedKey,
				key.String(),
				nil,
				fieldValue)
			if thisField == nil {
				continue
			}

			mapFields = append(mapFields, *thisField)

			if recurseValue != nil {
				// Recurse into the map value
				mapFields = append(mapFields, d.getStructFieldsRecursive(*recurseValue, *thisField)...)
			}
		}
		return mapFields
	}

	return []StructField{}
}

// Derive this once to use for checking implementation of encoding.TextUnmarshaler below.
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// Make a StructField with the given parameters.
// Return value sf will be nil if the field should be ignored.
// Return value recurseValue will be non-nil if the field should be recursed into (such as
// for maps and structs). recurseValue may be different than v, as unwrapping may have occurred
// (such as for pointers and interfaces).
func (d decoder) makeField(keyPrefix AliasedKey, name string, structTag *reflect.StructTag, v reflect.Value,
) (
	sf *StructField, recurseValue *reflect.Value,
) {
	if structTag != nil && d.codec.IsStructFieldIgnored(*structTag) {
		return nil, nil
	}

	kind := v.Kind()
	if kind == reflect.Ptr || kind == reflect.Interface {
		// Unwrap
		vElem := v.Elem()
		if vElem.IsValid() {
			// Recurse on the unwrapped value
			return d.makeField(keyPrefix, name, structTag, vElem)
		}
		// Otherwise it's nil and just fall through
	}

	sf = &StructField{}

	keyElem := AliasedKeyElem{name}
	if structTag != nil {
		if alias := d.codec.GetStructFieldAlias(*structTag); alias != "" {
			keyElem = append(keyElem, alias)
		}

		tagOpts := strings.Split(structTag.Get(d.tagName), ",")
		sf.Optional = (tagOpts[0] == "optional")
		if len(tagOpts) > 1 && tagOpts[1] != "" {
			sf.ExpectedType = tagOpts[1]
		}
	}

	// If the type of v implements encoding.TextUnmarshaler, then we expect a string
	if reflect.PtrTo(v.Type()).Implements(textUnmarshalerType) {
		sf.ExpectedType = "string"
	}

	sf.AliasedKey = make(AliasedKey, len(keyPrefix))
	copy(sf.AliasedKey, keyPrefix)
	sf.AliasedKey = append(sf.AliasedKey, keyElem)

	sf.Kind = kind.String()
	sf.Type = v.Type().String()

	recurseValue = nil
	if kind == reflect.Struct || kind == reflect.Map {
		recurseValue = &v
	}

	return sf, recurseValue
}
