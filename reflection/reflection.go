// Package reflection provides GetStructFields, allowing for the collection of structural
// information about structs or maps.
package reflection

import (
	"encoding"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Codec is an interface which must be implemented and passed to GetStructFields.
// It provides abstraction for the underlying config file type (like TOML or JSON).
type Codec interface {
	// Returns true if the struct tag indicates that the field is not unmarshaled (like `json:"-"`)
	IsStructFieldIgnored(st reflect.StructTag) bool

	// Returns the alias name of the field from the struct tag (like `json:"new_name"`)
	// or empty string if the field has no alias
	GetStructFieldAlias(st reflect.StructTag) string
}

// AliasedKeyElem is one component of an aliased key. Contains equivalent aliases for this component.
//
// Guarantee: The struct tag alias will be last.
type AliasedKeyElem []string

// Equal returns true if the two AliasedKeyElems match.
func (ake AliasedKeyElem) Equal(cmp AliasedKeyElem) bool {
	for _, alias1 := range ake {
		for _, alias2 := range cmp {
			// Do a case-insensitive comparison, since encoding/json and BurntSushi/toml do
			if strings.EqualFold(alias1, alias2) {
				return true
			}
		}
	}
	return false
}

// AliasedKey is a key that can be written multiple ways, all of which should match.
// Each element may have aliases. None of the elements will be empty.
type AliasedKey []AliasedKeyElem

// Equal returns true if the two AliasedKeys match.
func (ak AliasedKey) Equal(cmp AliasedKey) bool {
	if len(ak) != len(cmp) {
		return false
	}

	for i := range ak {
		akElem := ak[i]
		cmpElem := cmp[i]

		if !akElem.Equal(cmpElem) {
			return false
		}

		// Aliases matched for this element of the key; continue through the elements
	}

	return true
}

// HasPrefix returns true if prefix is a prefix of the key (or equal to it).
func (ak AliasedKey) HasPrefix(prefix AliasedKey) bool {
	if len(prefix) > len(ak) {
		return false
	}
	return ak[:len(prefix)].Equal(prefix)
}

// StructField holds information about field in a struct.
type StructField struct {
	// The key for the field within the struct (including possible aliase due to struct tags)
	AliasedKey AliasedKey

	// true if the field has been flagged as optional in the struct tag; false otherwise.
	Optional bool

	// reflect's Type() for the field. Like "time.Time".
	Type string
	// reflect's Kind() for the field. Like "struct".
	Kind string

	// If the strut tag contains an explicit type, it will be provided here.
	ExpectedType string

	// Pointer to the parent of the field (for non-roots)
	Parent *StructField
	// Pointers to the children of this field (for non-leafs)
	Children []*StructField

	// NOTE: If any fields are added, make sure to update the compareStructFields test helper.
}

// decoder holds the tag name and codec used by a call to GetStructFields
type decoder struct {
	tagName string
	codec   Codec
}

/*
GetStructFields returns information about the structure of obj. It can be passed either
a struct or a map. The non-exported fields of a struct are ignored.

tagName is the struct tag name that will be used to flag whether a field is optional and
if there is an explicit type that should be associated with it.

codec implements Codec and is used to determine if fields have an alias or should be
ignored. (I.e., with the `json:` or `toml:` struct tags.)

The returned slice is guaranteed to have branches before leaves.
*/
func GetStructFields(obj interface{}, tagName string, codec Codec) []*StructField {
	d := decoder{tagName, codec}
	return d.getStructFieldsRecursive(reflect.ValueOf(obj), nil)
}

// Recursion helper for GetStructFields.
// structValue starts out as the reflect.Value of the initial struct or map, and then gets
// the value of each field as the struct/map is walked.
// currField starts out as the zero value. After that it is the field that is being recursed
// into, since much of the field info (like name) is not available from the Value itself.
func (d decoder) getStructFieldsRecursive(structValue reflect.Value, currField *StructField) []*StructField {
	switch structValue.Kind() {
	case reflect.Ptr:
		// Unwrap and recurse
		structValue = structValue.Elem()
		if structValue.IsValid() {
			// currField and parent don't change as a result of unwrapping
			return d.getStructFieldsRecursive(structValue, currField)
		}

	// If it is a struct we walk each field
	case reflect.Struct:
		structFields := make([]*StructField, 0)
		for i := 0; i < structValue.NumField(); i++ {
			field := structValue.Field(i)
			fieldType := structValue.Type().Field(i)
			if fieldType.PkgPath != "" {
				// unexported; see https://golang.org/pkg/reflect/#StructField
				continue
			}

			var keyPrefix AliasedKey
			if currField != nil {
				keyPrefix = currField.AliasedKey
			}

			thisField, recurseValue := d.makeField(
				keyPrefix,
				fieldType.Name,
				&fieldType.Tag,
				field,
				currField) // the parent of this new field
			if thisField == nil {
				continue
			}

			structFields = append(structFields, thisField)

			if recurseValue != nil {
				// Recurse into the field
				structFields = append(structFields, d.getStructFieldsRecursive(*recurseValue, thisField)...)
			}
		}
		return structFields

	// Recurse into maps
	case reflect.Map:
		mapFields := make([]*StructField, 0)
		// We'll collect and sort the keys, mostly to make testing easier later (and
		// because there won't be so many fields that this is a performance problem).
		var keys []reflect.Value
		for _, key := range structValue.MapKeys() {
			keys = append(keys, key)
		}

		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

		for _, key := range keys {
			fieldValue := structValue.MapIndex(key)

			var keyPrefix AliasedKey
			if currField != nil {
				keyPrefix = currField.AliasedKey
			}

			thisField, recurseValue := d.makeField(
				keyPrefix,
				key.String(),
				nil,
				fieldValue,
				currField) // the parent of this new field
			if thisField == nil {
				continue
			}

			mapFields = append(mapFields, thisField)

			if recurseValue != nil {
				// Recurse into the map value
				mapFields = append(mapFields, d.getStructFieldsRecursive(*recurseValue, thisField)...)
			}
		}
		return mapFields
	}

	return []*StructField{}
}

// Derive this once to use for checking implementation of encoding.TextUnmarshaler below.
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// Make a StructField with the given parameters.
// Return value sf will be nil if the field should be ignored.
// Return value recurseValue will be non-nil if the field should be recursed into (such as
// for maps and structs). recurseValue may be different than v, as unwrapping may have occurred
// (such as for pointers and interfaces).
func (d decoder) makeField(keyPrefix AliasedKey, name string, structTag *reflect.StructTag, v reflect.Value, parent *StructField,
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
			return d.makeField(keyPrefix, name, structTag, vElem, parent)
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
	sf.Parent = parent
	if sf.Parent != nil {
		sf.Parent.Children = append(sf.Parent.Children, sf)
	}

	recurseValue = nil
	if kind == reflect.Struct || kind == reflect.Map {
		recurseValue = &v
	}

	return sf, recurseValue
}

// String is intended to be used for making example output more readable.
func (sf StructField) String() string {
	sb := strings.Builder{}

	sb.WriteString("StructField{\n")
	sb.WriteString(fmt.Sprintf("\tAliasedKey: %v\n", sf.AliasedKey))
	sb.WriteString(fmt.Sprintf("\tOptional: %v\n", sf.Optional))
	sb.WriteString(fmt.Sprintf("\tType: %v\n", sf.Type))
	sb.WriteString(fmt.Sprintf("\tKind: %v\n", sf.Kind))

	// Comparing example output will choke on the trailing space, so special-case the empty value
	if sf.ExpectedType != "" {
		sb.WriteString(fmt.Sprintf("\tExpectedType: %v\n", sf.ExpectedType))
	} else {
		sb.WriteString("\tExpectedType:\n")
	}

	if sf.Parent != nil {
		sb.WriteString(fmt.Sprintf("\tParent: %v\n", sf.Parent.AliasedKey))
	} else {
		sb.WriteString(fmt.Sprintf("\tParent: nil\n"))
	}

	if len(sf.Children) > 0 {
		sb.WriteString("\tChildren: {\n")
		for _, child := range sf.Children {
			sb.WriteString(fmt.Sprintf("\t\t%v\n", child.AliasedKey))
		}
		sb.WriteString("\t}\n")
	} else {
		sb.WriteString("\tChildren: {}\n")
	}

	sb.WriteString("}")

	return sb.String()
}
