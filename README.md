[![](https://img.shields.io/github/release/Psiphon-Inc/configloader-go.svg)](https://github.com/Psiphon-Inc/configloader-go/releases/latest) [![GoDoc](https://godoc.org/github.com/Psiphon-Inc/configloader-go?status.svg)](https://godoc.org/github.com/Psiphon-Inc/configloader-go) [![Actions Status](https://wdp9fww0r9.execute-api.us-west-2.amazonaws.com/production/badge/Psiphon-Inc/configloader-go)](https://wdp9fww0r9.execute-api.us-west-2.amazonaws.com/production/results/Psiphon-Inc/configloader-go)

# configloader-go

`configloader` is a Go library for loading config from multiple files (like `config.toml` and `config_override.toml`), with defaults and environment variable overrides. It provides the following features:
* Info on the provenance of each config field value -- which file the value came from, or if it was an env var override, or a default, or absent.
* Ability to flag fields as optional. And error will result if required fields are absent.
* Detection of vestigial fields in the config files -- fields which are unknown to the code.
* Ability to supply default field values.
* Environment variable field overriding.
* Additional expected-type checking, with detailed error messages. (Although this is of limited value over the checking in encoding/json and BurntSushi/toml).
* No more dependencies than you need.

See the [GoDoc documentation](https://godoc.org/github.com/Psiphon-Inc/configloader-go).

## Why not [Viper](https://github.com/spf13/viper)?

1. It pulls in 20+ dependencies, including config languages you aren't using. (We vendor all our dependencies, so this is especially glaring.)
2. It doesn't support multiple files.

## Installation

```
go get -u github.com/Psiphon-Inc/configloader-go
```

## Example

For a full sample, see the ["recommended" sample app](https://github.com/Psiphon-Inc/configloader-go/blob/master/examples/recommended/config/config.go).

```golang
import (
  "github.com/Psiphon-Inc/configloader-go"
  "github.com/Psiphon-Inc/configloader-toml"
)

type Config struct {
  Server struct{
    ListenPort string
  }

  Log struct {
    Level string
    Format string `conf:"optional"`
  }

  Stats struct {
    SampleCount int `toml:"sample_count"`
  }
}

envVarOverrides := []configloader.EnvOverride{
  {
    EnvVar: "SERVER_PORT",
    Key: configloader.Key{"Server", "ListenPort"},
  },
}

defaults := []configloader.Default{
  {
    Key: configloader.Key{"Log", "Level"},
    Val: "info",
  },
  {
    Key: configloader.Key{"Stats", "SampleCount"},
    Val: 1000,
  },
}

var config Config

metadata, err := configloader.Load(
  toml.Codec, // Specifies config file format
  configReaders, configReaderNames,
  defaults,
  envVarOverrides,
  &config)

// Record the config info. May help diagnose problems later.
log.Print(metadata.ConfigMap) // or log.Print(config)
log.Print(metadata.Provenances)
```

## Future work

* More examples:
  - Singleton config
  - Map config

* Type checking inside slices (and better slice handling generally)

* HCL (or HCL2) support. Note that we'll either need better top-level slice support, or
  specify a limitation of no-top-level slices (which are easy to get with HCL).

* Re-evaluate whether the type checking is worthwhile at all or if it should just be left
  to the unmarshaler.

## License

BSD 3-Clause License
