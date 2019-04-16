package psiconfig

import (
	"encoding"
	"reflect"
	"strings"
)

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

type aliasedKeyElem []string
type aliasedKey []aliasedKeyElem

type structField struct {
	aliasedKey   aliasedKey
	typ          string
	kind         string
	optional     bool
	expectedType string
}

func (sf structField) copy() structField {
	newSF := sf
	newSF.aliasedKey = make(aliasedKey, len(sf.aliasedKey))
	copy(newSF.aliasedKey, sf.aliasedKey)
	return newSF
}

func getStructFields(obj interface{}) []structField {
	return getStructFieldsRecursive(reflect.ValueOf(obj), structField{})
}

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

func makeField(keyPrefix aliasedKey, name string, structTag *reflect.StructTag, v reflect.Value) (sf *structField, recurseValue *reflect.Value) {
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
