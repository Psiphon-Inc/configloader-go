/*
 * BSD 3-Clause License
 * Copyright (c) 2019, Psiphon Inc.
 * All rights reserved.
 */

package configloader

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/Psiphon-Inc/configloader-go/reflection"
	"github.com/pkg/errors"
)

// TagName is used in struct tags like `conf:"optional"`. Can be modified if the caller desires.
var TagName = "conf"

// Codec is the interface that specific config file language support must implement.
// See the json and toml packages for examples.
type Codec interface {
	reflection.Codec

	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error

	// Codec-specific checks (OVER AND ABOVE decoder.fieldTypesConsistent).
	// For example, encoding.json makes all numbers float64.
	// noDeeper should be true if the structure should not be checked any deeper.
	// err must be non-nil if the types are _not_ consistent.
	// check and gold will never be nil.
	FieldTypesConsistent(check, gold *reflection.StructField) (noDeeper bool, err error)
}

// Key is a field path into a struct or map. For most cases it can contain the field names
// used in the result struct, or the aliases used in the config file.
// A struct path key might look like Key{"Stats", "SampleCount"}.
// A config alias key might look like Key{"stats", "sample_count"}.
type Key []string

// Convert k to a string appropriate for keying a map (so, unique and consistent).
func (k Key) String() string {
	// NOTE: If we support a config language that uses "." in keys, we'll have to make
	// this more robust/careful/complicated.
	return strings.Join(k, ".")
}

// MarshalText implements encoding.TextMarshaler. To be used with JSON logging (especially of Provenances).
func (k Key) MarshalText() (text []byte, err error) {
	return []byte(k.String()), nil
}

// EnvOverride indicates that a field should be overridden by an environment variable
// value, if it exists.
type EnvOverride struct {
	// The environment variable. Case-sensitive.
	EnvVar string

	// The key of the field that should be overridden.
	Key Key

	// A function to convert from the string obtained from the environment to the type
	// required by the field. For example:
	//   func(v string) interface{} {
	// 	   return strconv.Atoi(v)
	//   }
	Conv func(envString string) (interface{}, error)
}

// Default is used to provide a default value for a field if it is otherwise absent.
type Default struct {
	// The key of the field that will start with the default value.
	Key Key

	// The value the field should be given if it doesn't receive any other value.
	Val interface{}
}

// Provenance indicates the source that the value of a field ultimately came from.
type Provenance struct {
	// We store aliasedKey as well as Key for the purposes of accessing and printing by caller
	aliasedKey reflection.AliasedKey

	// The key of the field this is the provenance for.
	Key Key

	// The source of the value of the field. It can be one of the following:
	//   "path/to/file.toml": If the value came from a file and readerNames was provided to Load()
	//   "[0]": If the value came from a file and readerNames was not provided to Load()
	//   "[default]": If the field received the default value passed to Load()
	//   "[absent]": If the field was not set at all
	//   "$ENV_VAR_NAME": If the field value came from an environment variable override
	Src string
}

// Provenances provides the sources (provenances) for all of the fields in the resulting
// struct or map.
// It is good practice to log this value for later debugging help.
type Provenances []Provenance

// Metadata contains information about the loaded config.
type Metadata struct {
	structFields []*reflection.StructField
	absentFields []*reflection.StructField

	// A map version of the resulting config.
	// It is good practice to log either this map or the config struct for later debugging help,
	// BUT ONLY IF THEY DON'T CONTAIN SECRETS.
	// (If the result is already a map, this is identical.)
	ConfigMap map[string]interface{}

	// The sources of each config field.
	// It is good practice to log either this map or the config struct for later debugging help.
	Provenances Provenances
}

// IsDefined checks if the given key was defined in the loaded struct (including from
// defaults or environment variable overrides).
// Error is returned if the key is not valid for the result struct. (So error is never
// returned if the result is a map.)
func (md *Metadata) IsDefined(key ...string) (bool, error) {
	aliasedKey := aliasedKeyFromKey(key)

	// If the key is among the absent fields, then it's not defined.
	if _, ok := findStructField(md.absentFields, aliasedKey); ok {
		return false, nil
	}

	// If it's not absent and it is in the struct, then it is defined.
	if _, ok := findStructField(md.structFields, aliasedKey); ok {
		return true, nil
	}

	// If the result was a map rather than a struct, then we don't have structFields
	// or absentFields and we'll have to look in the map that was produced.
	if len(md.structFields) == 0 {
		currMap := md.ConfigMap
		for i := range key {
			if _, ok := currMap[key[i]]; !ok {
				return false, nil
			}

			// The key is in this level of the map.
			// If this is the last key elem, then we don't need to dig into the map
			// any deeper, otherwise we do.
			if i < len(key)-1 {
				var ok bool
				currMap, ok = currMap[key[i]].(map[string]interface{})
				if !ok {
					// Not a map, so can't dig deeper
					return false, nil
				}
			}
		}

		// We found all the pieces of the key in the map
		return true, nil
	}

	return false, errors.Errorf("key does not exist among known fields: %+v", md.structFields)
}

// Add or overwrite the provenance src for the given key
func (md *Metadata) setProvenance(k Key, src string) {
	ak := aliasedKeyFromKey(k)

	// Try to find the full aliased key
	if sf, ok := findStructField(md.structFields, ak); ok {
		ak = sf.AliasedKey
	}

	// See if the new provenance is already in the slice (possibly with an alias)
	for i := range md.Provenances {
		if ak.Equal(md.Provenances[i].aliasedKey) {
			// Already present; update
			md.Provenances[i].Src = src
			return
		}
	}

	// This is a new one
	prov := Provenance{
		aliasedKey: ak,
		Key:        keyFromAliasedKey(ak),
		Src:        src,
	}
	md.Provenances = append(md.Provenances, prov)
}

// String converts the provenance to a string. Useful for debugging, logging, or examples.
func (prov Provenance) String() string {
	return fmt.Sprintf("'%s':'%s'", prov.Key, prov.Src)
}

// String converts the provenances to a string. Useful for debugging, logging, or examples.
func (provs Provenances) String() string {
	// We want to print sorted by Key, so do that first
	sort.Slice(provs, func(i, j int) bool { return provs[i].Key.String() < provs[j].Key.String() })

	provStrings := make([]string, len(provs))
	for i := range provs {
		provStrings[i] = provs[i].String()
	}

	return fmt.Sprintf("{ %s }", strings.Join(provStrings, "; "))
}

// decoder captures the variables shared between many calls.
// This is mostly an effort to clean up the number of params of many helpers, and the
// ugliness of passing codec through helpers that don't actually use it directly to get it
// deeper helpers that do.
type decoder struct {
	codec Codec
}

// Load gathers config data from readers, defaults, and environment overrides, and
// populates result with the values. It provides log-able provenance information for each
// field in the metadata.
//
// codec implements config-file-type-specific helpers. It's possible to use a custom
// implementation, but you probably want to use one of the configloader-go sub-packages (like json or toml).
//
// readers will be used to populate the config. Later readers in the slice will take
// precedence and values from them will clobber the earlier.
//
// readerNames contains useful names for the readers. This is intended to be the filenames
// obtained from FindFiles(). This is partly a human-readable convenience for provenances
// and partly essential to know exactly which files were used (as FindFiles look across
// multiple search paths).
//
// defaults will be used to populate the result before any other sources.
//
// envOverrides is a mapping from environment variable name to config key path. These are
// applied after all other sources.
//
// result may be struct or map[string]interface{}.
//
// Some of the reasons an error may be returned:
//   - A required field is absent
//   - A field was found in the config sources that is not present in the result struct
//   - The type of a value in the config sources didn't match the expected type in the result struct
//   - One of the readers couldn't be read
//   - Some other codec unmarshaling problem
func Load(codec Codec, readers []io.Reader, readerNames []string, defaults []Default, envOverrides []EnvOverride, result interface{},
) (
	md Metadata, err error,
) {
	decoder := decoder{codec}

	if readerNames != nil && len(readerNames) != len(readers) {
		return md, errors.New("readerNames must be nil or the same length as readers")
	}

	reflectResult := reflect.ValueOf(result)
	if reflectResult.Kind() != reflect.Ptr {
		return md, errors.Errorf("result must be pointer; got %s", reflect.TypeOf(result))
	}
	if reflectResult.IsNil() {
		return md, errors.Errorf("result is nil %s", reflect.TypeOf(result))
	}

	_, resultIsMap := result.(*map[string]interface{})

	// Get info about the struct being populated. If result is actually a map and not a
	// struct, this will be empty.
	md.structFields = reflection.GetStructFields(result, TagName, codec)

	// We'll use this to build up the combined config map
	accumConfigMap := make(map[string]interface{})

	//
	// Defaults
	//

	// The presence of a default value for a field implies that the field is optional.
	// Update the structFields appropriately.
	defaultsMap := make(map[string]interface{})
	for _, dflt := range defaults {
		// If we're setting into a struct (vs a map), make sure the key is valid
		if !resultIsMap {
			sf, ok := findStructField(md.structFields, aliasedKeyFromKey(dflt.Key))
			if !ok {
				return md, errors.Errorf("defaults key not found in struct: %+v", dflt)
			}

			// Because a default was supplied, assume this field is optional
			sf.Optional = true

			// Convert the key into one that prefers aliases
			dflt.Key = keyFromAliasedKey(sf.AliasedKey)
		}

		if err := setMapByKey(defaultsMap, dflt.Key, dflt.Val, md.structFields); err != nil {
			return md, errors.Wrapf(err, "setMapByKey failed for default: %+v", dflt)
		}

		md.setProvenance(dflt.Key, "[default]")
	}

	if !resultIsMap {
		// We ignore absentFields for now. Just checking types and vestigials.
		_, err = decoder.verifyFieldsConsistency(
			reflection.GetStructFields(defaultsMap, TagName, codec), md.structFields)
		if err != nil {
			return md, errors.Wrapf(err, "verifyFieldsConsistency failed for defaults")
		}
	}

	// Merge the env map into the accum map (contributor updating happened above)
	decoder.mergeMaps(accumConfigMap, defaultsMap, md.structFields)

	//
	// Readers
	//

	// Get the config (file) data from the readers
	for i, r := range readers {
		readerName := fmt.Sprintf("[%d]", i)
		if len(readerNames) > i {
			readerName = readerNames[i]
		}

		b, err := ioutil.ReadAll(r)
		if err != nil {
			return md, errors.Wrapf(err, "ioutil.ReadAll failed for config reader '%s'", readerName)
		}

		var newConfigMap map[string]interface{}
		err = codec.Unmarshal(b, &newConfigMap)
		if err != nil {
			return md, errors.Wrapf(err, "codec.Unmarshal failed for config reader '%s'", readerName)
		}

		if !resultIsMap {
			// We ignore absentFields for now. Just checking types and vestigials.
			_, err = decoder.verifyFieldsConsistency(
				reflection.GetStructFields(newConfigMap, TagName, codec), md.structFields)
			if err != nil {
				return md, errors.Wrapf(err, "verifyFieldsConsistency failed for config reader '%s'", readerName)
			}
		}

		// Merge the new map into the accum map, and collect contributor info
		keysMerged := decoder.mergeMaps(accumConfigMap, newConfigMap, md.structFields)
		for _, k := range keysMerged {
			md.setProvenance(k, readerName)
		}
	}

	//
	// Environment variables
	//

	// Now add in the environment var overrides
	envMap := make(map[string]interface{})
	for _, eo := range envOverrides {
		// If we're setting into a struct (vs a map), make sure the key is valid
		if !resultIsMap {
			sf, ok := findStructField(md.structFields, aliasedKeyFromKey(eo.Key))
			if !ok {
				return md, errors.Errorf("envOverride key not found in struct: %+v", eo)
			}

			// Convert the key into one that prefers aliases
			eo.Key = keyFromAliasedKey(sf.AliasedKey)
		}

		valStr, ok := os.LookupEnv(eo.EnvVar)
		if !ok {
			continue
		}

		// If the caller provided a type converter, apply it now
		var valI interface{} = valStr
		if eo.Conv != nil {
			if valI, err = eo.Conv(valStr); err != nil {
				return md, errors.Wrapf(err, "conversion of env var string failed for envOverride: %+v", eo)
			}
		}

		if err := setMapByKey(envMap, eo.Key, valI, md.structFields); err != nil {
			return md, errors.Wrapf(err, "setMapByKey failed for envOverride: %+v", eo)
		}

		md.setProvenance(eo.Key, "$"+eo.EnvVar)
	}

	if !resultIsMap {
		// We ignore absentFields for now. Just checking types and vestigials.
		_, err = decoder.verifyFieldsConsistency(
			reflection.GetStructFields(envMap, TagName, codec), md.structFields)
		if err != nil {
			return md, errors.Wrapf(err, "verifyFieldsConsistency failed for env overrides")
		}
	}

	// Merge the env map into the accum map (contributor updating happened above)
	decoder.mergeMaps(accumConfigMap, envMap, md.structFields)

	//
	// Finalize
	//

	if resultIsMap {
		// There's nothing more to do. With a simple map, there's no such thing
		// as absent fields or required fields or field consistency.
		resultMap := result.(*map[string]interface{})
		if *resultMap == nil {
			*resultMap = make(map[string]interface{})
		}
		decoder.mergeMaps(*resultMap, accumConfigMap, md.structFields)
		md.ConfigMap = *resultMap
		return md, nil
	}

	// Verify fields one last time on the whole accumulated map, checking absent fields
	md.absentFields, err = decoder.verifyFieldsConsistency(
		reflection.GetStructFields(accumConfigMap, TagName, codec), md.structFields)
	if err != nil {
		// This shouldn't happen, since we've checked all the inputs into accumConfigMap
		return md, errors.Wrapf(err, "verifyFieldsConsistency failed for merged map")
	}

	// Set the provenance of absent fields, and detect if any required fields are missing
	var missingRequiredFields []*reflection.StructField
	for _, f := range md.absentFields {
		// We only record provenance for leafs
		if len(f.Children) == 0 {
			md.setProvenance(keyFromAliasedKey(f.AliasedKey), "[absent]")
		}

		// If a branch of the tree (struct or map) is optional and absent, then its
		// children will not be considered "required". But if that optional branch is
		// present, then its children must adhere to their own optional status.
		if f.Optional && len(f.Children) > 0 {
			// This field is absent, optional, and has children (so it's a branch).
			// Mark all of its children and grandchildren as optional as well, so they
			// don't get flagged as "required".
			// Note that if some children are themselves branches, when they get processed
			// their children will get marked Optional, and so on.
			for _, child := range f.Children {
				child.Optional = true
			}
		}

		if !f.Optional {
			missingRequiredFields = append(missingRequiredFields, f)
		}
	}
	if len(missingRequiredFields) > 0 {
		return md, errors.Errorf("missing required fields: %+v", missingRequiredFields)
	}

	// We now have a map populated with all of our data, including env overrides.
	// Marshal it and then re-unmarshal it into the destination struct.
	buf, err := codec.Marshal(accumConfigMap)
	if err != nil {
		return md, errors.Wrap(err, "Re-marshaling accumulated config map failed")
	}
	err = codec.Unmarshal(buf, result)
	if err != nil {
		return md, errors.Wrap(err, "Failed to unmarshal result struct")
	}

	// In order to populate Metadata.ConfigMap, we need to marshal our final struct and
	// then unmarshal it into a map. The reason we can't just use accumConfigMap is that
	// there may have been values already set into the result struct and we can't get at
	// them without going the long way.
	buf, err = codec.Marshal(result)
	if err != nil {
		return md, errors.Wrap(err, "Failed to marshal final result struct")
	}
	err = codec.Unmarshal(buf, &md.ConfigMap)
	if err != nil {
		return md, errors.Wrap(err, "Failed to unmarshal final config map")
	}

	return md, nil
}

func setMapByKey(m map[string]interface{}, k Key, v interface{}, structFields []*reflection.StructField) error {
	aliasedKey := aliasedKeyFromKey(k)

	// We'll try to find an AliasedKey from the provided struct fields (if any). If we
	// can't find a full match, we'll look for prefixes, as we might be settings a
	// map-within-a-struct, that only has struct fields up to a certain point, and that
	// we still want to match.
	keyPrefix := k
	for len(keyPrefix) > 0 {
		if sf, ok := findStructField(structFields, aliasedKeyFromKey(keyPrefix)); ok {
			// Found a match. Combine this prefix with the rest of the original key.
			aliasedKey = append(sf.AliasedKey, aliasedKey[len(sf.AliasedKey):]...)
			break
		}

		// Didn't find it; try the next shorter prefix
		keyPrefix = keyPrefix[:len(keyPrefix)-1]
	}

	currMap := m
	for i := range aliasedKey {
		// The input key might be using struct field names rather than aliases, which
		// will result in the final unmarshaling not finding those fields. So we'll
		// prefer to use the alias, which is the last element of AliasedKeyElem.
		keyElem := aliasedKey[i][len(aliasedKey[i])-1]

		// If the field already exists in the map, use the key/field that's there,
		// otherwise build the map at keyElem.
		for currMapKey := range currMap {
			if aliasedKey[i].Equal(reflection.AliasedKeyElem{currMapKey}) {
				keyElem = currMapKey
				break
			}
		}

		// We're either at the leaf or at an intermediate node.
		if i == len(aliasedKey)-1 {
			// Leaf
			currMap[keyElem] = v
			break
		}

		// Intermediate. Make sure it's a map.
		if currMap[keyElem] == nil {
			// Either it doesn't exist or it exists and is nil
			currMap[keyElem] = make(map[string]interface{})
		} else if _, ok := currMap[keyElem].(map[string]interface{}); !ok {
			// The map key exists, but is not itself a map. Not okay.
			return errors.Errorf("Map subtree is not a map; full key: %+v; map subtree key: %+v; map: %+v", k, k[:i+1], m)
		}

		// Get the sub-map for the next iteration of the loop
		currMap = currMap[keyElem].(map[string]interface{})
	}

	return nil
}

// Merge src into dst, overwriting values.
// The keys of the leaves merged are returned.
func (d decoder) mergeMaps(dst, src map[string]interface{}, structFields []*reflection.StructField) (keysMerged []Key) {
	// Get all the fields of the src map
	srcStructFields := reflection.GetStructFields(src, TagName, d.codec)
	dstStructFields := reflection.GetStructFields(dst, TagName, d.codec)

	for i, srcField := range srcStructFields {
		if srcField.Kind == "map" {
			// We only want to explicitly copy leaves. A map can be a leaf if it has no
			// children. Luckily, the ordering guarantee of structFields is such that
			// the very next key will be a child, if one exists.
			// Additionally, we don't want clobber existing maps with empty ones.
			if (i+1 < len(srcStructFields)) && srcStructFields[i+1].AliasedKey.HasPrefix(srcField.AliasedKey) {
				// This map is not a leaf, as the next field is a child
				continue
			}

			if _, existsInDst := findStructField(dstStructFields, srcField.AliasedKey); existsInDst {
				// This map (or at least a field at this key) already exists in dst.
				// We won't clobber it.
				continue
			}
		}

		// Find the value to merge for this field
		var val interface{}
		var key Key
		currMap := src
		for i, keyElem := range srcField.AliasedKey {
			key = append(key, keyElem[0])
			// Plain maps don't have multiple aliases, so keyElem[0] is sufficient
			if i == len(srcField.AliasedKey)-1 {
				// This is the last part of the key, so we have the value
				val = currMap[keyElem[0]]
				break
			}
			currMap = currMap[keyElem[0]].(map[string]interface{})
		}

		// This is a leaf
		setMapByKey(dst, key, val, structFields)
		keysMerged = append(keysMerged, key)
	}

	return keysMerged
}

// Checks three things:
// 1. There's nothing in check that's not in gold (because that indicates a vestigial
//    field in the config).
// 2. The field types match.
// 3. Absent fields (both required and optional). Return this, but don't error on it.
func (d decoder) verifyFieldsConsistency(check, gold []*reflection.StructField) (absentFields []*reflection.StructField, err error) {
	// Start by treating all the gold fields as absent, then remove them as we hit them
	absentFieldsCandidates := make([]*reflection.StructField, len(gold))
	copy(absentFieldsCandidates, gold)

	var skipPrefixes []reflection.AliasedKey

CheckFieldsLoop:
	for _, checkField := range check {
		for _, skipPrefix := range skipPrefixes {
			if checkField.AliasedKey.HasPrefix(skipPrefix) {
				continue CheckFieldsLoop
			}
		}

		goldField, ok := findStructField(gold, checkField.AliasedKey)
		if !ok {
			return nil, errors.Errorf("field in config not found in struct: %+v", checkField)
		}

		// Remove goldField from absentFieldsCandidates
		for i := range absentFieldsCandidates {
			if absentFieldsCandidates[i].AliasedKey.Equal(goldField.AliasedKey) {
				absentFieldsCandidates = append(absentFieldsCandidates[:i], absentFieldsCandidates[i+1:]...)
				break
			}
		}

		noDeeper, err := d.fieldTypesConsistent(checkField, goldField)
		if err != nil {
			return nil, errors.Wrapf(err, "field types not consistent; got %+v, want %+v", checkField, goldField)
		}

		if noDeeper {
			skipPrefixes = append(skipPrefixes, checkField.AliasedKey)
		}
	}

	// Keys skipped due to skipPrefix do not cound as "absent", so remove any matches
	// from absentFieldsCandidates that didn't get processed above.
	absentFields = make([]*reflection.StructField, 0)
AbsentSkipLoop:
	for _, absent := range absentFieldsCandidates {
		for _, skipPrefix := range skipPrefixes {
			if absent.AliasedKey.HasPrefix(skipPrefix) {
				continue AbsentSkipLoop
			}
		}
		// Doesn't match any skip prefixes
		absentFields = append(absentFields, absent)
	}

	return absentFields, nil
}

// Check if the field types in check are consistent with those in gold.
// It is assumed that check is from a map and gold is from a struct.
// If noDeeper is true on return, the caller should not recurse any deeper into this
// field's structure.
func (d decoder) fieldTypesConsistent(check, gold *reflection.StructField) (noDeeper bool, err error) {
	/*
		Examples:
		- time.Time implements encoding.TextUnmarshaler, so expectedType will be "string"
	*/

	if gold.ExpectedType != "" {
		// If a type is specified, then it must match exactly
		if check.Type != gold.ExpectedType && check.Kind != gold.ExpectedType {
			return false, errors.Errorf("check field type/kind does not match gold expected type; check:%+v; gold:%+v", check, gold)
		}

		// When we hit an expected type, we don't want to go any deeper into the keys
		// along this branch of the tree.
		// For example, a struct that supports UnmarshalText (with explicit type "string")
		// might have sub-fields that shouldn't be included in the consistency check.
		return true, nil
	}

	if gold.Kind == "map" {
		// We won't have any structure to compare any deeper, so...
		noDeeper = true
	}

	// Exact match
	if check.Type == gold.Type || check.Type == gold.Kind {
		return noDeeper, nil
	}

	// E.g., if there's a "*string" in the gold and a "string" in the check, that's fine
	if gold.Type == "*"+check.Type {
		return noDeeper, nil
	}

	if gold.Kind == "struct" && check.Kind == "map" {
		return noDeeper, nil
	}

	if gold.Kind == "map" && check.Type == "map[string]interface {}" {
		// If there's a concrete-type map (like map[string]int) in the gold, it will end
		// up being map[string]interface{} in the check, so we have to be loose with the
		// comparison.
		return noDeeper, nil
	}

	// We'll treat different int sizes as equivalent.
	// If values are too big for specified types, an error will occur
	// when unmarshalling.
	if strings.HasPrefix(gold.Kind, "int") && strings.HasPrefix(check.Kind, "int") {
		return noDeeper, nil
	}

	// We'll treat different float sizes as equivalent.
	// If values are too big for specified types, an error will occur
	// when unmarshalling.
	if strings.HasPrefix(gold.Kind, "float") && strings.HasPrefix(check.Kind, "float") {
		return noDeeper, nil
	}

	// We don't check types inside a slice.
	// TODO: Type checking inside slices.
	if gold.Kind == "slice" && check.Kind == "slice" {
		return true, nil
	}

	// See if there are any codec-specific checks to make this okay
	noDeeper, err = d.codec.FieldTypesConsistent(check, gold)
	if err == nil {
		return noDeeper, nil
	}
	// err is set, but we'll create our own for consistency

	return false, errors.Errorf("check field type/kind does not match gold type/kind; check:%+v; gold:%+v", check, gold)
}

func findStructField(fields []*reflection.StructField, targetKey reflection.AliasedKey) (*reflection.StructField, bool) {
	for i := range fields {
		fieldPtr := fields[i]
		if len(fieldPtr.AliasedKey) != len(targetKey) {
			// Can't possibly match
			continue
		}

		if targetKey.Equal(fieldPtr.AliasedKey) {
			// We found the field
			return fieldPtr, true
		}
	}

	// We exhausted the search without a match
	return nil, false
}

func aliasedKeyFromKey(key Key) reflection.AliasedKey {
	result := make(reflection.AliasedKey, len(key))
	for i := range key {
		result[i] = reflection.AliasedKeyElem{key[i]}
	}
	return result
}

func keyFromAliasedKey(ak reflection.AliasedKey) Key {
	result := make(Key, len(ak))
	for i := range ak {
		// Prefer the last component, as it's the alias
		result[i] = ak[i][len(ak[i])-1]
	}
	return result
}
