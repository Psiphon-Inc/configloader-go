package main

import (
	"fmt"

	"github.com/Psiphon-Inc/configloader-go"
	"github.com/Psiphon-Inc/configloader-go/toml"
)

func main() {
	// The first file must exist, but none of the others.
	fileLocations := []configloader.FileLocation{
		{
			Filename: "config.toml",
			// The search paths are in order of preference.
			SearchPaths: []string{".", "/etc/config"},
		},
		{
			Filename: "config_override.toml",
			// Don't look elsewhere for an override
			SearchPaths: []string{"."},
		},
	}

	readers, closers, readerNames, err := configloader.FindFiles(fileLocations...)
	if err != nil {
		panic(fmt.Sprintf("Failed to find config files: %v", err))
	}

	defer func() {
		for _, r := range closers {
			r.Close()
		}
	}()

	var config map[string]interface{}

	metadata, err := configloader.Load(
		toml.Codec, // Specifies config file format
		readers, readerNames,
		nil, // No defaults
		nil, // No env var overrides
		&config)
	if err != nil {
		panic(fmt.Sprintf("configloader.Load failed: %v", err))
	}

	// DEBUG
	fmt.Printf("Config: %+v\n", config)
	fmt.Printf("Provenances: %+v\n", metadata.Provenances)

	// Then start on our server, listening on port config["listen_port"].(int)
}
