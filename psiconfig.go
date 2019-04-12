package psiconfig

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

// Like https://godoc.org/github.com/BurntSushi/toml#Key
type Key []string

// Convert k to a string appropriate for keying a map (so, unique and consistent).
func (k Key) String() string {
	return strings.Join(k, ".")
}

// Create one of our Keys from a toml.Key.
func newKeyFromTomlKey(tk toml.Key) Key {
	return Key(tk)
}

type EnvOverride struct {
	EnvVar string
	Key    Key // MUST refer to the key as used in the TOML (as opposed to the struct)
	Conv   func(envString string) interface{}
}

type Contributions map[string]string

type Metadata struct {
	tomlMD           *toml.MetaData
	configStructKeys []aliasedKey
	Contributions    Contributions
}

// IsDefined tests whether the given key was defined -- either in the TOML sources or as
// an environment variable.
// key may refer to a field in the config struct or in the TOML, which may differ by case
// or through the use of a `toml:"name"`.
// If the given key cannot be mapped to a field in the struct, an error will be returned.
func (md *Metadata) IsDefined(key ...string) (bool, error) {
	// This check is complicated by the fact that Go struct exported fields must be uppercase,
	// but TOML fields will typically -- but not necessarily -- be lower case, and `toml:"name"`
	// can be used to map to anything at all. And we're allowing the input to either or
	// both or a mix.

	if len(key) == 0 {
		return false, nil
	}

	// First find the corresponding struct key
	var matchingStructKey *aliasedKey
StructSearchLoop:
	for _, structKey := range md.configStructKeys {
		if len(structKey) < len(key) {
			// Can't possibly match
			continue
		}

		// Loop through the components of the supplied key to see if this structKey matches
	KeySearchLoop:
		for i := range key {
			keyElem := key[i]
			structKeyElemAliases := structKey[i]

			// structKey elems may have multiple names (because of `toml:"name"`)
			for _, alias := range structKeyElemAliases {
				// Do a case-insensitive comparison, since BurntSushi/toml does
				if strings.EqualFold(alias, keyElem) {
					// We found a matching alias for this key elem
					continue KeySearchLoop
				}
			}

			// No alias matched; it can't be this struct key
			continue StructSearchLoop
		}

		// This struct key matched. But it might actually be longer than the one
		// specified by the caller, so truncate to the match length.
		truncatedStructKey := structKey[:len(key)]
		matchingStructKey = &truncatedStructKey
		break
	}

	if matchingStructKey == nil {
		// We didn't find any matching struct key. This indicates an incorrect call.
		return false, errors.Errorf("given key matched no field of the config struct: %#v", key)
	}

	// We know what struct key to use; try to find it in the TOML keys
TOMLKeySearchLoop:
	for _, tomlKey := range md.tomlMD.Keys() {
		if len(tomlKey) < len(*matchingStructKey) {
			// Can't possibly match
			continue
		}

	StructKeySearchLoop:
		for i := range *matchingStructKey {
			structKeyElemAliases := (*matchingStructKey)[i]
			tomlKeyElem := tomlKey[i]

			for _, alias := range structKeyElemAliases {
				if strings.EqualFold(alias, tomlKeyElem) {
					// We found a matching alias for this key elem
					continue StructKeySearchLoop
				}
			}

			// No alias matched; this can't be the tomlKey
			continue TOMLKeySearchLoop
		}

		// tomlKey matches matchingStructKey, so our answer is "true"
		return true, nil
	}

	return false, nil
}

// readers will be used to populate the config. Later readers in the slice will take precedence and clobber the earlier.
// envOverrides is a map from environment variable key to config key path (config.DB.Password is ["DB", "Password"]).
// Each of those envvars will result in overriding a key's value in the resulting struct.
// absentKeys can be used to determine if defaults should be applied (as zero values might be valid and
// not indicate absence).
func Load(readers []io.Reader, readerNames []string, envOverrides []EnvOverride, result interface{},
) (md Metadata, err error) {

	if readerNames != nil && len(readerNames) != len(readers) {
		return md, errors.New("readerNames must be nil or the same length as readers")
	}

	md.Contributions = make(map[string]string)

	accumConfigMap := make(map[string]interface{})

	for i, r := range readers {
		readerName := strconv.Itoa(i)
		if len(readerNames) > i {
			readerName = readerNames[i]
		}

		var newConfigMap map[string]interface{}
		tomlMetadata, err := toml.DecodeReader(r, &newConfigMap)
		if err != nil {
			return md, errors.Wrapf(err, "toml.Decode failed for config reader '%s'", readerName)
		}

		// Merge the new map into the accum map, and collect contributor info
		for _, tk := range tomlMetadata.Keys() {
			k := newKeyFromTomlKey(tk)
			if setMapLeafFromMap(newConfigMap, accumConfigMap, k) {
				md.Contributions[k.String()] = readerName
			}
		}
	}

	// Now add in the environment var overrides
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

		if err := setMapByKey(accumConfigMap, eo.Key, valI); err != nil {
			return md, errors.Wrapf(err, "setMapByKey failed for EnvOverride: %+v", eo)
		}

		md.Contributions[eo.Key.String()] = "$" + eo.EnvVar
	}

	// We now have a map populated with all of our data, including env overrides. Now put
	// it back into TOML and then re-decode it into the destination struct.
	stringWriter := strings.Builder{}
	tomlEnc := toml.NewEncoder(&stringWriter)
	err = tomlEnc.Encode(accumConfigMap)
	if err != nil {
		return md, errors.Wrap(err, "Re-encoding config map failed")
	}

	tomlString := stringWriter.String()
	tomlMetadata, err := toml.Decode(tomlString, result)
	if err != nil {
		return md, errors.Wrap(err, "Failed to decode re-encoded config")
	}
	md.tomlMD = &tomlMetadata

	// Detect unused keys and error; indicates vestigial settings, a bad key rename, or incorrect envvar override
	if len(tomlMetadata.Undecoded()) > 0 {
		return md, errors.Errorf("unused keys found: %+v", tomlMetadata.Undecoded())
	}

	md.configStructKeys = structKeys(result)

	return md, nil
}

// setMapLeafFromMap copies the first non-map value along k from fromMap to toMap.
// Any intermediate structure missing from toMap will be created.
// Returns true if the value at the full key was copied, and false if no copy was made or
// if the copy happened on a subset of the key (because a non-map was encountered).
// (This return behaviour is to help us with recording "contributions".)
// The key might actually be a path leading through an array-of-maps -- traversal will
// stop at the array (i.e., it stops at any non-map).
// It will panic if the types are inconsistent between the maps.
func setMapLeafFromMap(fromMap, toMap map[string]interface{}, k Key) bool {
	if len(k) == 0 {
		// This shouldn't actually happen, but handle it gracefully
		return false
	} else if len(k) == 1 {
		// We're done recursing. We still only copy if we're a non-map.
		fromVal := fromMap[k[0]]
		if _, ok := fromVal.(map[string]interface{}); ok {
			// fromVal is a map, so we don't copy it
			return false
		}

		toMap[k[0]] = fromVal
		return true
	}

	// We have more of the key to process.

	// If the next step of fromMap is no longer a map, there's nothing more to do
	nextFromMap, ok := fromMap[k[0]].(map[string]interface{})
	if !ok {
		return false
	}

	// Intermediate maps might not yet exist in toMap, so create if necessary.
	subtreeWasAbsent := false
	subtreeWasNil := false
	if subtree, ok := toMap[k[0]]; !ok {
		subtreeWasAbsent = true
		toMap[k[0]] = make(map[string]interface{})
	} else if subtree == nil {
		subtreeWasNil = true
		toMap[k[0]] = make(map[string]interface{})
	}

	// There's the possibility that the key exists in toMap but is of the wrong type.
	// This means the contents of our sources are inconsistent. Panic.
	nextToMap, ok := toMap[k[0]].(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("Inconsitent config values between sources for key ending with: %#v", k))
	}

	// Recuse into the map tree
	if !setMapLeafFromMap(nextFromMap, nextToMap, k[1:]) {
		// No leaf was set. Revert our subtree additions, if we made any.
		if subtreeWasAbsent {
			delete(toMap, k[0])
		} else if subtreeWasNil {
			toMap[k[0]] = nil
		}

		return false
	}

	return true
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

/*
func FilesToUse(filenames, searchPaths []string) []string {
	multoml supports taking a bunch of filenames to use -- ["config.toml", "config_override.toml"] --
	and search paths in which to look for them, in order of preference -- [".", "/etc/whatever"] -- and
	indicates which absolute filepaths should be used for config loading.
	This should be replicated in this standalone function which can be used before the loading starts.
	See https://github.com/Psiphon-Inc/multoml/blob/master/multoml.go#L139
}
*/
