package psiconfig

package psiconfig

import (
	"io"
	"os"
 	"github.com/BurntSushi/toml"
 	"github.com/pkg/errors"
)

// Like https://godoc.org/github.com/BurntSushi/toml#Key
type Key []string

// Convert k to a string appropriate for keying a map (so, unique and consistent).
func (k Key) string() string {
	return fmt.Sprintf("%#v", k)
}

// Create one of our Keys from a toml.Key.
func newKeyFromTomlKey(tk toml.Key) Key {
	return Key(tk)
}

// TODO: Do we want detailed info about what keys came from which files, and which from envvars? Like, so we can log and try to diagnose weirdnesses.

// readers will be used to populate the config. Later readers in the slice will take precedence and clobber the earlier.
// envOverrides is a map from environment variable key to config key path (config.DB.Password is ["DB", "Password"]).
// Each of those envvars will result in overriding a key's value in the resulting struct.
// absentKeys can be used to determine if defaults should be applied (as zero values might be valid and
// not indicate absence).
func Load(readers []io.Reader, readerNames []string, envOverrides map[string]Key, result interface{},
	) (absentKeys []Key, contributions map[string]string, err error) {

	contributions = make(map[string]string)
	keysFound := make(map[string]struct{})

	if readerNames != nil && len(readerNames) != len(readers) {
		return errors.New("readerNames must be nil or the same length as readers")
	}

	for i, r := range readers {
		// Fields found in this decoding will clobber the fields from previous decodings;
		// but fields _not_ in this decoding will _not_ affect fields from previous decodings.
		tomlMetadata, err := toml.DecodeReader(r, result)
		if err != nil {
			return errors.Wrapf(err, "toml.Decode failed for config reader #%d", i)
		}

		// Detect unused keys and error; indicates vestigial settings or a bad key rename
		if len(tomlMetadata.Undecoded()) > 0 {
			return errors.Errorf("unused keys found in config reader #%d: %+v", i, tomlMetadata.Undecoded())
		}

		for tk := range tomlMetadata.Keys() {
			// We want to use our own key type, so we can have a stable stringified form to compare against later.
			k := newFromTomlKey(tk)
			kString := k.string()
			keysFound[kString] = struct{}

			if readerNames != nil {
				contributions[kString] = readerNames[i]
			} else {
				contributions[kString] = strconv.Itoa(i)
			}
		}
	}

	Use Reflect to walk through the v struct. For each key path, use keysFound to
	decide whether to add it to absent keys.
	Also check if the key path is present in envOverrides
	and if so do an os.LookupEnv and override the struct value. (Easier said than done. Env just gives
     	strings, so there is going to be some type trickery needed. Maybe a bit type switch? Surely
     	just primitives supported. But I am pretty certain it is possible.)

	return nil
}

func FilesToUse(filenames, searchPaths []string) []string {
	multoml supports taking a bunch of filenames to use -- ["config.toml", "config_override.toml"] --
	and search paths in which to look for them, in order of preference -- [".", "/etc/whatever"] -- and
	indicates which absolute filepaths should be used for config loading.
	This should be replicated in this standalone function which can be used before the loading starts.
	See https://github.com/Psiphon-Inc/multoml/blob/master/multoml.go#L139
}
