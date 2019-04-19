package psiconfig

import (
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/Psiphon-Inc/psiphon-go-config/reflection"
	"github.com/pkg/errors"
)

// Used in struct tags like `conf:"optional"`. Can be modified if the caller desires.
var TagName = "conf"

type Codec interface {
	reflection.Codec

	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error

	// Codec-specific checks (OVER AND ABOVE decoder.fieldTypesConsistent).
	// For example, encoding.json makes all numbers float64.
	// noDeeper should be true if the structure should not be checked any deeper.
	// err must be non-nil if the types are _not_ consistent.
	FieldTypesConsistent(check, gold reflection.StructField) (noDeeper bool, err error)
}

type Key []string

// Convert k to a string appropriate for keying a map (so, unique and consistent).
func (k Key) String() string {
	return strings.Join(k, ".")
}

type EnvOverride struct {
	EnvVar string
	Key    Key
	Conv   func(envString string) interface{}
}

type Contributions map[string]string

type Metadata struct {
	structFields []reflection.StructField
	absentFields []reflection.StructField
	configMap    *map[string]interface{}

	// TODO: comment, mention how to print
	Contributions Contributions
}

// TODO: Comment
// - for maps, never returns error
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
	if len(md.structFields) == 0 && md.configMap != nil {
		currMap := *md.configMap
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

type decoder struct {
	codec Codec
}

// readers will be used to populate the config. Later readers in the slice will take precedence and clobber the earlier.
// envOverrides is a map from environment variable key to config key path (config.DB.Password is ["DB", "Password"]).
// Each of those envvars will result in overriding a key's value in the resulting struct.
// absentKeys can be used to determine if defaults should be applied (as zero values might be valid and
// not indicate absence).
// result may be struct or map
func Load(codec Codec, readers []io.Reader, readerNames []string, envOverrides []EnvOverride, result interface{},
) (md Metadata, err error) {
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

	// If result is a map, clear it out, otherwise our field checking will be broken.
	// Consistency checks are not possible when the result is a map rather than a struct,
	// so record that it is for later branching.
	resultIsMap := false
	if resultMap, ok := (result).(*map[string]interface{}); ok {
		for k := range *resultMap {
			delete(*resultMap, k)
		}

		resultIsMap = true
	}

	// Get info about the struct being populated. If result is actually a map and not a
	// struct, this will be empty.
	md.structFields = reflection.GetStructFields(result, TagName, codec)

	md.Contributions = make(map[string]string)

	// We'll use this to build up the combined config map
	accumConfigMap := make(map[string]interface{})

	// Get the config (file) data from the readers
	for i, r := range readers {
		readerName := strconv.Itoa(i)
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
		keysMerged := mergeMaps(accumConfigMap, newConfigMap, nil)
		for _, k := range keysMerged {
			md.Contributions[k.String()] = readerName
		}
	}

	// Now add in the environment var overrides
	envMap := make(map[string]interface{})
	for _, eo := range envOverrides {
		// If we're setting into a struct (vs a map), make sure the key is valid
		if !resultIsMap {
			if _, ok := findStructField(md.structFields, aliasedKeyFromKey(eo.Key)); !ok {
				return md, errors.Errorf("envOverride key not found in struct: %+v", eo)
			}
		}

		valStr, ok := os.LookupEnv(eo.EnvVar)
		if !ok {
			continue
		}

		// If the caller provided a type converter, apply it now
		var valI interface{} = valStr
		if eo.Conv != nil {
			valI = eo.Conv(valStr)
		}

		if err := setMapByKey(envMap, eo.Key, valI, md.structFields); err != nil {
			return md, errors.Wrapf(err, "setMapByKey failed for envOverride: %+v", eo)
		}

		md.Contributions[eo.Key.String()] = "$" + eo.EnvVar
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
	mergeMaps(accumConfigMap, envMap, nil)

	if resultIsMap {
		// There's nothing more to do. With a simple map, there's no such thing
		// as absent fields or required fields or field consistency.
		// So we'll just return the map we've built up.
		resultMap := (result).(*map[string]interface{})
		*resultMap = make(map[string]interface{})
		mergeMaps(*resultMap, accumConfigMap, nil)
		md.configMap = resultMap
		return md, nil
	}

	// Verify fields one last time on the whole accumulated map, checking absent fields
	md.absentFields, err = decoder.verifyFieldsConsistency(
		reflection.GetStructFields(accumConfigMap, TagName, codec), md.structFields)
	if err != nil {
		// This shouldn't happen, since we've checked all the inputs into accumConfigMap
		return md, errors.Wrapf(err, "verifyFieldsConsistency failed for merged map")
	}

	var missingRequiredFields []reflection.StructField
	for _, f := range md.absentFields {
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
		return md, errors.Wrap(err, "Re-marshaling config map failed")
	}

	err = codec.Unmarshal(buf, result)
	if err != nil {
		return md, errors.Wrap(err, "Failed to decode re-encoded config")
	}

	return md, nil
}

func setMapByKey(m map[string]interface{}, k Key, v interface{}, structFields []reflection.StructField) error {
	aliasedKey := aliasedKeyFromKey(k)

	// We'll try to find a full aliasedKey from the provided struct fields (if any)
	sf, ok := findStructField(structFields, aliasedKeyFromKey(k))
	if ok {
		aliasedKey = sf.AliasedKey
	}

	currMap := m
	for i := range aliasedKey {
		keyElem := k[i]

		// If the field already exists in the map, use the key/field that's there,
		// otherwise build the map at keyElem.
		for currMapKey := range currMap {
			if aliasedKeyElemsMatch(aliasedKey[i], reflection.AliasedKeyElem{currMapKey}) {
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

// Merge src into dst, overwriting values. key must be nil on first call (the param
// is used for recursive calls).
func mergeMaps(dst, src map[string]interface{}, keyPrefix Key) (keysMerged []Key) {
	for k, v := range src {
		thisKey := append(keyPrefix, k)

		if srcMap, ok := v.(map[string]interface{}); ok {
			// Sub-map; recurse
			dstMap, ok := dst[k].(map[string]interface{})
			if !ok {
				dstMap = make(map[string]interface{})
				dst[k] = dstMap
			}

			keysMerged = append(keysMerged, mergeMaps(dstMap, srcMap, thisKey)...)
		} else {
			dst[k] = v
			keysMerged = append(keysMerged, thisKey)
		}
	}

	return keysMerged
}

// Checks three things:
// 1. There's nothing in check that's not in gold (because that indicates a vestigial
// field in the config).
// 2. The field types match.
// 3. Absent fields (required or optional). Return this, but don't error on it.
func (d decoder) verifyFieldsConsistency(check, gold []reflection.StructField) (absentFields []reflection.StructField, err error) {
	// Start by treating all the gold fields as absent, then remove them as we hit them
	absentFieldsCandidates := make([]reflection.StructField, len(gold))
	copy(absentFieldsCandidates, gold)

	var skipPrefixes []reflection.AliasedKey

CheckFieldsLoop:
	for _, checkField := range check {
		for _, skipPrefix := range skipPrefixes {
			if aliasedKeyPrefixMatch(checkField.AliasedKey, skipPrefix) {
				continue CheckFieldsLoop
			}
		}

		goldField, ok := findStructField(gold, checkField.AliasedKey)
		if !ok {
			return nil, errors.Errorf("field in config not found in struct: %+v", checkField)
		}

		// Remove goldField from absentFieldsCandidates
		for i := range absentFieldsCandidates {
			if aliasedKeysMatch(absentFieldsCandidates[i].AliasedKey, goldField.AliasedKey) {
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
	absentFields = make([]reflection.StructField, 0)
AbsentSkipLoop:
	for _, absent := range absentFieldsCandidates {
		for _, skipPrefix := range skipPrefixes {
			if aliasedKeyPrefixMatch(absent.AliasedKey, skipPrefix) {
				continue AbsentSkipLoop
			}
		}
		// Doesn't match any skip prefixes
		absentFields = append(absentFields, absent)
	}

	return absentFields, nil
}

// It is assumed that check is from a map and gold is from a struct.
func (d decoder) fieldTypesConsistent(check, gold reflection.StructField) (noDeeper bool, err error) {
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

	// Exact match
	if check.Type == gold.Type || check.Type == gold.Kind {
		return false, nil
	}

	// E.g., if there's a "*string" in the gold and a "string" in the check, that's fine
	if gold.Type == "*"+check.Type {
		return false, nil
	}

	if gold.Kind == "struct" && check.Kind == "map" {
		return false, nil
	}

	// We'll treat different int sizes as equivalent.
	// If values are too big for specified types, an error will occur
	// when unmarshalling.
	if strings.HasPrefix(gold.Kind, "int") && strings.HasPrefix(check.Kind, "int") {
		return false, nil
	}

	// We'll treat different float sizes as equivalent.
	// If values are too big for specified types, an error will occur
	// when unmarshalling.
	if strings.HasPrefix(gold.Kind, "float") && strings.HasPrefix(check.Kind, "float") {
		return false, nil
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

func aliasedKeysMatch(ak1, ak2 reflection.AliasedKey) bool {
	if len(ak1) != len(ak2) {
		return false
	}

	for i := range ak1 {
		ak1Elem := ak1[i]
		ak2Elem := ak2[i]

		if !aliasedKeyElemsMatch(ak1Elem, ak2Elem) {
			return false
		}

		// Aliases matched for this element of the key; continue through the elements
	}

	return true
}

func aliasedKeyElemsMatch(elem1, elem2 reflection.AliasedKeyElem) bool {
	for _, alias1 := range elem1 {
		for _, alias2 := range elem2 {
			// Do a case-insensitive comparison, since encoding/json and BurntSushi/toml do
			if strings.EqualFold(alias1, alias2) {
				return true
			}
		}
	}
	return false
}

func aliasedKeyPrefixMatch(ak, prefix reflection.AliasedKey) bool {
	if len(prefix) > len(ak) {
		return false
	}
	return aliasedKeysMatch(ak[:len(prefix)], prefix)
}

func findStructField(fields []reflection.StructField, targetKey reflection.AliasedKey) (reflection.StructField, bool) {
	for _, field := range fields {
		if len(field.AliasedKey) != len(targetKey) {
			// Can't possibly match
			continue
		}

		if aliasedKeysMatch(targetKey, field.AliasedKey) {
			// We found the field
			return field, true
		}
	}

	// We exhausted the search without a match
	return reflection.StructField{}, false
}

func aliasedKeyFromKey(key Key) reflection.AliasedKey {
	result := make(reflection.AliasedKey, len(key))
	for i := range key {
		result[i] = reflection.AliasedKeyElem{key[i]}
	}
	return result
}

/*
func FilesToUse(filenames, searchPaths []string) []string {
	multoml supports taking a bunch of filenames to use -- ["config.toml", "config_override.toml"] --
	and search paths in which to look for them, in order of preference -- [".", "/etc/whatever"] -- and
	indicates which absolute filepaths should be used for config loading.
	This should be replicated in this standalone function which can be used before the loading starts.
	See https://github.com/Psiphon-Inc/multoml/blob/master/multoml.go#L139
}
*/
