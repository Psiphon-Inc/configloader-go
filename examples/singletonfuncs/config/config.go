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

var conf Config

func Init() error {
	//
	// Load non-secret config
	//

	// The first file must exist, but none of the others.
	fileLocations := []configloader.FileLocation{
		{
			Filename: "config_nonsecret.toml",
			// The search paths are in order of preference.
			SearchPaths: []string{".", "/etc/config"},
		},
		{
			Filename: "config_nonsecret_override.toml",
			// Don't look elsewhere for an override
			SearchPaths: []string{"."},
		},
	}

	nonsecretReaders, nonsecretClosers, nonsecretReaderNames, err := configloader.FindFiles(fileLocations...)
	if err != nil {
		return errors.Wrap(err, "configloader.FindFiles failed for non-secret files")
	}

	defer func() {
		for _, r := range nonsecretClosers {
			r.Close()
		}
	}()

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
		defaults,
		nil, // No env var overrides
		&conf.nonsecret)
	if err != nil {
		return errors.Wrap(err, "configloader.Load failed for non-secret config")
	}

	//
	// Load secret config
	//

	fileLocations = []configloader.FileLocation{
		{
			Filename:    "config_secret.toml",
			SearchPaths: []string{".", "/etc/config"},
		},
		{
			Filename:    "config_override.toml",
			SearchPaths: []string{"."},
		},
	}

	secretReaders, secretClosers, secretReaderNames, err := configloader.FindFiles(fileLocations...)
	if err != nil {
		return errors.Wrap(err, "FindFiles failed for secret files")
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
		nil, // No defaults
		envOverrides,
		&conf.secret)
	if err != nil {
		return errors.Wrap(err, "configloader.Load failed for secret config")
	}

	//
	// Post-process fields
	//

	// CORS.appUserAgentsSet is derived from CORS.AppUserAgents
	conf.nonsecret.CORS.appUserAgentsSet = make(map[string]bool)
	for _, ua := range conf.nonsecret.CORS.AppUserAgents {
		conf.nonsecret.CORS.appUserAgentsSet[ua] = true
	}

	return nil
}

type ConfigProvenances struct {
	Nonsecret configloader.Provenances
	Secret    configloader.Provenances
}

func Provenances() ConfigProvenances {
	return ConfigProvenances{
		Nonsecret: conf.nonsecretMD.Provenances,
		Secret:    conf.secretMD.Provenances,
	}
}

func Map() map[string]interface{} {
	// Don't provide secret values
	return conf.nonsecretMD.ConfigMap
}

func LogLevel() string {
	return conf.nonsecret.Log.Level
}

func CORSUserAgentAllowed(ua string) bool {
	return conf.nonsecret.CORS.appUserAgentsSet[ua]
}

func StatsSampleCount() int {
	return conf.nonsecret.Stats.SampleCount
}

func DBPassword() string {
	return conf.secret.DB.Password
}
