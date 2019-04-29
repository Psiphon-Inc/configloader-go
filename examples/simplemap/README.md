# `configloader` Example: Simple Map

This example simply loads config into a `map[string]interface{}`. Access can then be as raw or as mediated as you want. Note that this is _not_ the [recommended usage](https://github.com/Psiphon-Inc/configloader-go/tree/master/examples/recommended), as it skips initialization-time checks and introduces the possibility of more runtime errors.

If you don't need any of the specific features of configloader (override config files, defaults, environment variable overrides), then you should probably just use the unmarshaler (encoding/json or BurntSushi/toml) directly.
