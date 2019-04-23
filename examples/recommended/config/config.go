package config

import (
	"github.com/Psiphon-Inc/configloader-go"
	"github.com/Psiphon-Inc/configloader-go/toml"
	"github.com/pkg/errors"
)

type nonsecretConfig struct {
	Log struct {
		Level string
	}

	CORS struct {
		AppUserAgents []string `toml:"app_user_agents"`

		// Non-public field to store the converted data type
		appUserAgentsSet map[string]bool
	}

	Stats struct {
		SampleCount int
	}
}

type secretConfig struct {
	DB struct {
		Password string
	}
}

type Config struct {
	nonsecret   nonsecretConfig
	nonsecretMD configloader.Metadata
	secret      secretConfig
	secretMD    configloader.Metadata
}

func New() (*Config, error) {
	var conf Config

	//
	// Load non-secret config
	//

	// The first file must exist, but none of the others.
	filenames := []string{"config_nonsecret.toml", "config_nonsecret_override.toml"}
	// The search paths are in order of preference.
	searchPaths := []string{".", "/etc/config"}

	nonsecretReaders, nonsecretClosers, nonsecretReaderNames, err := configloader.FindConfigFiles(filenames, searchPaths)
	if err != nil {
		return nil, errors.Wrap(err, "FindConfigFiles failed for non-secret files")
	}

	defer func() {
		for _, r := range nonsecretClosers {
			r.Close()
		}
	}()

	// Default values can be set like this, or after loading, as below
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

	conf.nonsecretMD, err = configloader.Load(
		toml.Codec, // Specifies config file format
		nonsecretReaders, nonsecretReaderNames,
		nil, // No env var overrides
		defaults,
		&conf.nonsecret)
	if err != nil {
		return nil, errors.Wrap(err, "configloader.Load failed for non-secret config")
	}

	//
	// Load secret config
	//

	filenames = []string{"config_secret.toml", "config_secret_override.toml"}

	secretReaders, secretClosers, secretReaderNames, err := configloader.FindConfigFiles(filenames, searchPaths)
	if err != nil {
		return nil, errors.Wrap(err, "FindConfigFiles failed for secret files")
	}

	defer func() {
		for _, r := range secretClosers {
			r.Close()
		}
	}()

	var envOverrides = []configloader.EnvOverride{
		{
			EnvVar: "DB_PASSWORD",
			Key:    configloader.Key{"DB", "Password"},
		},
	}

	conf.secretMD, err = configloader.Load(
		toml.Codec,
		secretReaders, secretReaderNames,
		envOverrides,
		nil, // No defaults
		&conf.secret)
	if err != nil {
		return nil, errors.Wrap(err, "configloader.Load failed for secret config")
	}

	//
	// Post-process fields
	//

	// CORS.appUserAgentsSet is derived from CORS.AppUserAgents
	conf.nonsecret.CORS.appUserAgentsSet = make(map[string]bool)
	for _, ua := range conf.nonsecret.CORS.AppUserAgents {
		conf.nonsecret.CORS.appUserAgentsSet[ua] = true
	}

	return &conf, nil
}

type Provenances struct {
	Nonsecret configloader.Provenances
	Secret    configloader.Provenances
}

func (c *Config) Provenances() Provenances {
	return Provenances{
		Nonsecret: c.nonsecretMD.Provenances,
		Secret:    c.secretMD.Provenances,
	}
}

func (c *Config) Map() map[string]interface{} {
	// Don't provide secret values
	return c.nonsecretMD.ConfigMap
}

func (c *Config) LogLevel() string {
	return c.nonsecret.Log.Level
}

func (c *Config) CORSUserAgentAllowed(ua string) bool {
	return c.nonsecret.CORS.appUserAgentsSet[ua]
}

func (c *Config) StatsSampleCount() int {
	return c.nonsecret.Stats.SampleCount
}

func (c *Config) DBPassword() string {
	return c.secret.DB.Password
}
