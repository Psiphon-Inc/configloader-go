package configloader

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Psiphon-Inc/configloader-go/reflection"
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

// MarshalText implements encoding.TextMarshaler. To be used with logging (especially of Provenances).
func (k Key) MarshalText() (text []byte, err error) {
	return []byte(k.String()), nil
}

type EnvOverride struct {
	EnvVar string
	Key    Key
	Conv   func(envString string) interface{}
}

type Default struct {
	Key Key
	Val interface{}
}

type Provenance struct {
	// We store aliasedKey as well as Key for the purposes of accessing and printing by caller
	aliasedKey reflection.AliasedKey
	Key        Key
	Src        string
}

type Provenances []Provenance

type Metadata struct {
	structFields []reflection.StructField
	absentFields []reflection.StructField

	// A map version of the config
	ConfigMap map[string]interface{}

	// The source for each config field
	Provenances Provenances
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

func (md *Metadata) setProvenance(k Key, src string) {
	ak := aliasedKeyFromKey(k)

	// Try to find the full aliased key
	if sf, ok := findStructField(md.structFields, ak); ok {
		ak = sf.AliasedKey
	}

	// See if the new provenance is already in the slice (possibly with an alias)
	for i := range md.Provenances {
		if aliasedKeysMatch(ak, md.Provenances[i].aliasedKey) {
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

func (prov Provenance) String() string {
	return fmt.Sprintf("'%s':'%s'", prov.Key, prov.Src)
}

func (provs Provenances) String() string {
	// We want to print sorted by Key, so do that first
	sort.Slice(provs, func(i, j int) bool { return provs[i].Key.String() < provs[j].Key.String() })

	provStrings := make([]string, len(provs))
	for i := range provs {
		provStrings[i] = provs[i].String()
	}

	return fmt.Sprintf("{ %s }", strings.Join(provStrings, "; "))
}

type decoder struct {
	codec Codec
}

// readers will be used to populate the config. Later readers in the slice will take precedence and clobber the earlier.
// envOverrides is a mapping from environment variable key to config key path (config.DB.Password is Key{"DB", "Password"}).
// defaults is a set of default values for keys.
// Each of those envvars will result in overriding a key's value in the resulting struct.
// absentKeys can be used to determine if defaults should be applied (as zero values might be valid and
// not indicate absence).
// result may be struct or map
func Load(codec Codec, readers []io.Reader, readerNames []string, envOverrides []EnvOverride, defaults []Default, result interface{},
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
			valI = eo.Conv(valStr)
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

	var missingRequiredFields []reflection.StructField
	for _, f := range md.absentFields {
		if !f.Optional {
			missingRequiredFields = append(missingRequiredFields, f)
		}

		md.setProvenance(keyFromAliasedKey(f.AliasedKey), "[absent]")
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

func setMapByKey(m map[string]interface{}, k Key, v interface{}, structFields []reflection.StructField) error {
	aliasedKey := aliasedKeyFromKey(k)

	// We'll try to find a full aliasedKey from the provided struct fields (if any)
	sf, ok := findStructField(structFields, aliasedKeyFromKey(k))
	if ok {
		aliasedKey = sf.AliasedKey
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

// Merge src into dst, overwriting values.
// The keys of the leaves merged are returned.
func (d decoder) mergeMaps(dst, src map[string]interface{}, structFields []reflection.StructField) (keysMerged []Key) {
	// Get all the fields of the src map
	srcStructFields := reflection.GetStructFields(src, TagName, d.codec)
	dstStructFields := reflection.GetStructFields(dst, TagName, d.codec)

	for i, srcField := range srcStructFields {
		if srcField.Kind == "map" {
			// We only want to explicitly copy leaves. A map can be a leaf if it has no
			// children. Luckily, the ordering guarantee of structFields is such that
			// the very next key will be a child, if one exists.
			// Additionally, we don't want clobber existing maps with empty ones.
			if (i+1 < len(srcStructFields)) && aliasedKeyPrefixMatch(srcStructFields[i+1].AliasedKey, srcField.AliasedKey) {
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

		noDeeper, err := d.fieldTypesConsistent(checkField, *goldField)
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

func findStructField(fields []reflection.StructField, targetKey reflection.AliasedKey) (*reflection.StructField, bool) {
	for i := range fields {
		fieldPtr := &fields[i]
		if len(fieldPtr.AliasedKey) != len(targetKey) {
			// Can't possibly match
			continue
		}

		if aliasedKeysMatch(targetKey, fieldPtr.AliasedKey) {
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
