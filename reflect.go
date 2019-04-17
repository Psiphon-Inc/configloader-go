package psiconfig

import (
	"encoding"
	"reflect"
	"strings"
)

// One component of an aliased key. Contains equivalent aliases for this component.
type aliasedKeyElem []string

// A key that can be written multiple ways, all of which should match. Each element may have aliases.
type aliasedKey []aliasedKeyElem

// Information about field in a struct
type structField struct {
	aliasedKey   aliasedKey
	typ          string
	kind         string
	optional     bool
	expectedType string
}

/*
getStructFields returns the Exported fields found in obj.
The returned slice is guaranteed to have branches before leaves.

Example with struct:
	type S struct {
		F     string `toml:"eff" psiconfig:"optional"`
		Inner struct {
			InnerF int
		}
		Time time.Time // implements encoding.UnmarshalText

		unexported string
	}

	var s S
	fmt.Printf("%+v\n", getStructFields(s))
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
	fmt.Printf("%+v\n", getStructFields(m))
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
func getStructFields(obj interface{}) []structField {
	return getStructFieldsRecursive(reflect.ValueOf(obj), structField{})
}

// Recursion helper for getStructFields.
func getStructFieldsRecursive(structValue reflect.Value, currField structField) []structField {
	switch structValue.Kind() {
	case reflect.Ptr:
		// Unwrap and recurse
		structValue = structValue.Elem()
		if structValue.IsValid() {
			return getStructFieldsRecursive(structValue, currField)
		}

	// If it is a struct we walk each field
	case reflect.Struct:
		structFields := make([]structField, 0)
		for i := 0; i < structValue.NumField(); i++ {
			field := structValue.Field(i)
			fieldType := structValue.Type().Field(i)
			if fieldType.PkgPath != "" {
				// unexported; see https://golang.org/pkg/reflect/#StructField
				continue
			}

			thisField, recurseValue := makeField(
				currField.aliasedKey,
				fieldType.Name,
				&fieldType.Tag,
				field)
			if thisField == nil {
				continue
			}

			structFields = append(structFields, *thisField)

			if recurseValue != nil {
				// Recurse into the field
				structFields = append(structFields, getStructFieldsRecursive(*recurseValue, *thisField)...)
			}
		}
		return structFields

	// Recurse into maps
	case reflect.Map:
		mapFields := make([]structField, 0)
		for _, key := range structValue.MapKeys() {
			fieldValue := structValue.MapIndex(key)

			thisField, recurseValue := makeField(
				currField.aliasedKey,
				key.String(),
				nil,
				fieldValue)
			if thisField == nil {
				continue
			}

			mapFields = append(mapFields, *thisField)

			if recurseValue != nil {
				// Recurse into the map value
				mapFields = append(mapFields, getStructFieldsRecursive(*recurseValue, *thisField)...)
			}
		}
		return mapFields
	}

	return []structField{}
}

// Derive this once to use for checking implementation of encoding.TextUnmarshaler below.
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// Make a structField with the given parameters.
// Return value sf will be nil if the field should be ignored.
// Return value recurseValue will be non-nil if the field should be recursed into (such as
// for maps and structs). recurseValue may be different than v, as unwrapping may have occurred
// (such as for pointers and interfaces).
func makeField(keyPrefix aliasedKey, name string, structTag *reflect.StructTag, v reflect.Value,
) (
	sf *structField, recurseValue *reflect.Value,
) {
	if structTag != nil && isStructFieldIgnored(*structTag) {
		return nil, nil
	}

	kind := v.Kind()
	if kind == reflect.Ptr || kind == reflect.Interface {
		// Unwrap
		vElem := v.Elem()
		if vElem.IsValid() {
			// Recurse on the unwrapped value
			return makeField(keyPrefix, name, structTag, vElem)
		}
		// Otherwise it's nil and just fall through
	}

	sf = &structField{}

	keyElem := aliasedKeyElem{name}
	if structTag != nil {
		if alias := getStructFieldAlias(*structTag); alias != "" {
			keyElem = append(keyElem, alias)
		}

		tagOpts := strings.Split(structTag.Get(structTagName), ",")
		sf.optional = (tagOpts[0] == "optional")
		if len(tagOpts) > 1 && tagOpts[1] != "" {
			sf.expectedType = tagOpts[1]
		}
	}

	// If the type of v implements encoding.TextUnmarshaler, then we expect a string
	if reflect.PtrTo(v.Type()).Implements(textUnmarshalerType) {
		sf.expectedType = "string"
	}

	sf.aliasedKey = make(aliasedKey, len(keyPrefix))
	copy(sf.aliasedKey, keyPrefix)
	sf.aliasedKey = append(sf.aliasedKey, keyElem)

	sf.kind = kind.String()
	sf.typ = v.Type().String()

	recurseValue = nil
	if kind == reflect.Struct || kind == reflect.Map {
		recurseValue = &v
	}

	return sf, recurseValue
}

// Returns true if the struct tag indicates that the field should not be inspected
func isStructFieldIgnored(st reflect.StructTag) bool {
	// Both stdlib/json and BurntSushi/toml use "-" to indicate an ignored field.
	// If we add more supported config formats, we can add more interpretations here.
	for _, typ := range []string{"toml", "json"} {
		typeTag := st.Get(typ)
		if typeTag == "-" {
			return true
		}
	}
	return false
}

// Returns empty string if the field has no alias
func getStructFieldAlias(st reflect.StructTag) string {
	for _, typ := range []string{"toml", "json"} {
		typeTag := st.Get(typ)
		if typeTag != "" {
			return strings.Split(typeTag, ",")[0]
		}
	}
	return ""
}
