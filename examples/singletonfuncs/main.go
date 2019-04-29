package main

import (
	"fmt"

	"github.com/Psiphon-Inc/configloader-go/examples/singletonfuncs/config"
	"github.com/pkg/errors"
)

func main() {
	err := config.Init()
	if err != nil {
		panic(errors.Wrap(err, "config.Init failed"))
	}

	/*
		Now the config can be accessed like:
		config.LogLevel()
		config.DBPassword()
	*/

	// DEBUG
	fmt.Printf("Config: %+v\n", config.Map())
	fmt.Printf("Provenances: %+v\n", config.Provenances())
}
