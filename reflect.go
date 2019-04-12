package psiconfig

// Initially adapted from: https://gist.github.com/hvoecking/10772475

import (
	"reflect"
	"strings"
)

type aliasedKeyElem []string
type aliasedKey []aliasedKeyElem

func structKeys(obj interface{}) []aliasedKey {
	return structKeysRecursive(reflect.ValueOf(obj), aliasedKey{})
}

func structKeysRecursive(original reflect.Value, currKey aliasedKey) []aliasedKey {
	switch original.Kind() {
	// The first cases handle nested structures and walk them recursively

	// If it is a pointer we need to unwrap and call once again
	case reflect.Ptr:
		// To get the actual value of the original we have to call Elem()
		// At the same time this unwraps the pointer so we don't end up in
		// an infinite recursion
		originalValue := original.Elem()
		// Check if the pointer is nil
		if !originalValue.IsValid() {
			// Treat this as a leaf. Check if it's exported.
			if original.Type().PkgPath() == "" { // see https://golang.org/pkg/reflect/#Type
				return []aliasedKey{currKey}
			}
			return nil
		}
		// Continue along the pointer
		return structKeysRecursive(originalValue, currKey)

	// If it is an interface (which is very similar to a pointer), do basically the
	// same as for the pointer. Though a pointer is not the same as an interface so
	// note that we have to call Elem() after creating a new object because otherwise
	// we would end up with an actual pointer
	case reflect.Interface:
		// Get rid of the wrapping interface
		originalValue := original.Elem()

		// Check if the inteface is nil
		if !originalValue.IsValid() {
			// Treat this as a leaf. Check if it's exported.
			if original.Type().PkgPath() == "" { // see https://golang.org/pkg/reflect/#Type
				return []aliasedKey{currKey}
			}
			return nil
		}
		return structKeysRecursive(originalValue, currKey)

	// If it is a struct we walk each field
	case reflect.Struct:
		keys := make([]aliasedKey, 0, original.NumField())
		for i := 0; i < original.NumField(); i++ {
			field := original.Field(i)
			fieldType := original.Type().Field(i)
			if fieldType.PkgPath != "" { // see https://golang.org/pkg/reflect/#StructField
				// Not exported, skip
				continue
			}

			// We will record the field name and any tag alias
			keyElem := aliasedKeyElem{fieldType.Name}
			if tag, ok := fieldType.Tag.Lookup("toml"); ok && tag != "-" {
				// Add an alias
				keyElem = append(keyElem, strings.Split(tag, ",")[0])
			}

			keys = append(keys, structKeysRecursive(field, append(currKey, keyElem))...)
		}
		return keys

	// And everything else is a leaf
	default:
		// Check if the field is exported
		if original.Type().PkgPath() == "" { // see https://golang.org/pkg/reflect/#Type
			return []aliasedKey{currKey}
		}
		return nil
	}
}
