package psiconfig

import (
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const structTagName = "psiconfig"

type Codec struct {
	Marshal   func(v interface{}) ([]byte, error)
	Unmarshal func(data []byte, v interface{}) error
}

type Key []string

// Convert k to a string appropriate for keying a map (so, unique and consistent).
func (k Key) String() string {
	return strings.Join(k, ".")
}

type EnvOverride struct {
	EnvVar string
	// MUST refer to the key as used in the TOML (as opposed to the struct).
	// (If this is problematic, it can be changed, with effort.)
	Key  Key
	Conv func(envString string) interface{}
}

type Contributions map[string]string

type Metadata struct {
	structFields  []structField
	absentFields  []structField
	Contributions Contributions
}

func (md *Metadata) IsDefined(key ...string) (bool, error) {
	aliasedKey := aliasedKeyFromKey(key)
	_, ok := findStructField(md.absentFields, aliasedKey)
	if ok {
		return false, nil
	}

	_, ok = findStructField(md.structFields, aliasedKey)
	if ok {
		return true, nil
	}

	return false, errors.Errorf("key does not exist among known fields: %+v", md.structFields)
}

// readers will be used to populate the config. Later readers in the slice will take precedence and clobber the earlier.
// envOverrides is a map from environment variable key to config key path (config.DB.Password is ["DB", "Password"]).
// Each of those envvars will result in overriding a key's value in the resulting struct.
// absentKeys can be used to determine if defaults should be applied (as zero values might be valid and
// not indicate absence).
// result may be struct or map
func Load(codec Codec, readers []io.Reader, readerNames []string, envOverrides []EnvOverride, result interface{},
) (md Metadata, err error) {

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

	// If result is a map, clear it out, otherwise our field checking will be broken
	if _, ok := (result).(*map[string]interface{}); ok {
		m := make(map[string]interface{})
		result = &m
	}

	// Get info about the struct being populated. If result is actually a map and not a
	// struct, this will be empty.
	md.structFields = getStructFields(result)

	md.Contributions = make(map[string]string)

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

		s := string(b)
		_ = s

		var newConfigMap map[string]interface{}
		err = codec.Unmarshal(b, &newConfigMap)
		if err != nil {
			return md, errors.Wrapf(err, "codec.Unmarshal failed for config reader '%s'", readerName)
		}

		// We ignore absentFields for now. Just checking types and vestigials.
		_, err = verifyFieldsConsistency(getStructFields(newConfigMap), md.structFields)
		if err != nil {
			return md, errors.Wrapf(err, "verifyFieldsConsistency failed for config reader '%s'", readerName)
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
		valStr, ok := os.LookupEnv(eo.EnvVar)
		if !ok {
			continue
		}

		// If the caller provided a type converter, apply it now
		var valI interface{} = valStr
		if eo.Conv != nil {
			valI = eo.Conv(valStr)
		}

		if err := setMapByKey(envMap, eo.Key, valI); err != nil {
			return md, errors.Wrapf(err, "setMapByKey failed for EnvOverride: %+v", eo)
		}

		md.Contributions[eo.Key.String()] = "$" + eo.EnvVar
	}

	// We ignore absentFields for now. Just checking types and vestigials.
	_, err = verifyFieldsConsistency(getStructFields(envMap), md.structFields)
	if err != nil {
		return md, errors.Wrapf(err, "verifyFieldsConsistency failed for env overrides")
	}

	// Merge the env map into the accum map, and collect contributor info
	mergeMaps(accumConfigMap, envMap, nil)

	// Verify fields one last time on the whole accumulated map, checking absent fields
	md.absentFields, err = verifyFieldsConsistency(getStructFields(accumConfigMap), md.structFields)
	if err != nil {
		// This shouldn't happen, since we've checked all the inputs into accumConfigMap
		return md, errors.Wrapf(err, "verifyFieldsConsistency failed for merged map")
	}

	var missingRequiredFields []structField
	for _, f := range md.absentFields {
		if !f.optional {
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

func setMapByKey(m map[string]interface{}, k Key, v interface{}) error {
	if len(k) == 0 || m == nil {
		return errors.Errorf("bad state")
	} else if len(k) == 1 {
		// We're on the last step of our search
		m[k[0]] = v
		return nil
	}

	// We're at an intermediate step in the map
	if m[k[0]] == nil {
		m[k[0]] = make(map[string]interface{})
	} else if _, ok := m[k[0]].(map[string]interface{}); !ok {
		// The map key exists, but is not itself a map. Not okay.
		return errors.Errorf("Map subtree is not a map; key suffix: %#v; map subtree: %#v", k, m)
	}

	return setMapByKey(m[k[0]].(map[string]interface{}), k[1:], v)
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
func verifyFieldsConsistency(check, gold []structField) (absentFields []structField, err error) {
	// Start by treating all the gold fields as absent, then remove them as we hit them
	absentFieldsCandidates := make([]structField, len(gold))
	copy(absentFieldsCandidates, gold)

	var skipPrefixes []aliasedKey

CheckFieldsLoop:
	for _, checkField := range check {
		for _, skipPrefix := range skipPrefixes {
			if aliasedKeyPrefixMatch(checkField.aliasedKey, skipPrefix) {
				continue CheckFieldsLoop
			}
		}

		goldField, ok := findStructField(gold, checkField.aliasedKey)
		if !ok {
			return nil, errors.Errorf("field in config not found in struct: %+v", checkField)
		}

		// Remove goldField from absentFieldsCandidates
		for i := range absentFieldsCandidates {
			if aliasedKeysMatch(absentFieldsCandidates[i].aliasedKey, goldField.aliasedKey) {
				absentFieldsCandidates = append(absentFieldsCandidates[:i], absentFieldsCandidates[i+1:]...)
				break
			}
		}

		noDeeper, err := fieldTypesConsistent(checkField, goldField)
		if err != nil {
			return nil, errors.Wrapf(err, "field types not consistent; got %+v, want %+v", checkField, goldField)
		}

		if noDeeper {
			skipPrefixes = append(skipPrefixes, checkField.aliasedKey)
		}
	}

	// Keys skipped due to skipPrefix do not cound as "absent", so remove any matches
	// from absentFieldsCandidates that didn't get processed above.
	absentFields = make([]structField, 0)
AbsentSkipLoop:
	for _, absent := range absentFieldsCandidates {
		for _, skipPrefix := range skipPrefixes {
			if aliasedKeyPrefixMatch(absent.aliasedKey, skipPrefix) {
				continue AbsentSkipLoop
			}
		}
		// Doesn't match any skip prefixes
		absentFields = append(absentFields, absent)
	}

	return absentFields, nil
}

// It is assumed that check is from a map and gold is from a struct.
func fieldTypesConsistent(check, gold structField) (noDeeper bool, err error) {
	/*
		Examples:
		- time.Time implements encoding.TextUnmarshaler, so expectedType will be "string"


	*/

	if gold.expectedType != "" {
		// If a type is specified, then it must match exactly
		if check.typ != gold.expectedType && check.kind != gold.expectedType {
			return false, errors.Errorf("check field type/kind does not match gold expected type; check:%+v; gold:%+v", check, gold)
		}

		// When we hit an expected type, we don't want to go any deeper into the keys
		// along this branch of the tree.
		// For example, a struct that supports UnmarshalText (with explicit type "string")
		// might have sub-fields that shouldn't be included in the consistency check.
		return true, nil
	}

	// Exact match
	if check.typ == gold.typ || check.typ == gold.kind {
		return false, nil
	}

	// E.g., if there's a "*string" in the gold and a "string" in the check, that's fine
	if gold.typ == "*"+check.typ {
		return false, nil
	}

	if gold.kind == "struct" && check.kind == "map" {
		return false, nil
	}

	// We'll treat different int sizes as equivalent.
	// If values are too big for specified types, an error will occur
	// when unmarshalling.
	if strings.HasPrefix(gold.kind, "int") && strings.HasPrefix(check.kind, "int") {
		return false, nil
	}

	// We'll treat different float sizes as equivalent.
	// If values are too big for specified types, an error will occur
	// when unmarshalling.
	if strings.HasPrefix(gold.kind, "float") && strings.HasPrefix(check.kind, "float") {
		return false, nil
	}

	// We don't check types inside a slice.
	// TODO: Type checking inside slices.
	if gold.kind == "slice" && check.kind == "slice" {
		return true, nil
	}

	return false, errors.Errorf("check field type/kind does not match gold expected type; check:%+v; gold:%+v", check, gold)
}

func aliasedKeysMatch(ak1, ak2 aliasedKey) bool {
	if len(ak1) != len(ak2) {
		return false
	}

	for i := range ak1 {
		ak1Elem := ak1[i]
		ak2Elem := ak2[i]

		for _, ak1Alias := range ak1Elem {
			for _, ak2Alias := range ak2Elem {
				// Do a case-insensitive comparison, since BurntSushi/toml does
				if strings.EqualFold(ak1Alias, ak2Alias) {
					goto AliasMatched
				}
			}
		}
		// We failed to match any of the aliases
		return false

	AliasMatched:
		// Aliases matched for this element of the key; continue through the elements
	}

	return true
}

func aliasedKeyPrefixMatch(ak, prefix aliasedKey) bool {
	return aliasedKeysMatch(ak[:len(prefix)], prefix)
}

func findStructField(fields []structField, targetKey aliasedKey) (structField, bool) {
	for _, field := range fields {
		if len(field.aliasedKey) != len(targetKey) {
			// Can't possibly match
			continue
		}

		if aliasedKeysMatch(targetKey, field.aliasedKey) {
			// We found the field
			return field, true
		}
	}

	// We exhausted the search without a match
	return structField{}, false
}

func aliasedKeyFromKey(key Key) aliasedKey {
	result := make(aliasedKey, len(key))
	for i := range key {
		result[i] = aliasedKeyElem{key[i]}
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
