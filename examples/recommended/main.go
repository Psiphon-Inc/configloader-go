package main

import (
	"fmt"

	"github.com/Psiphon-Inc/configloader-go/examples/recommended/api"
	"github.com/Psiphon-Inc/configloader-go/examples/recommended/config"
	"github.com/Psiphon-Inc/configloader-go/examples/recommended/db"
	"github.com/Psiphon-Inc/configloader-go/examples/recommended/log"
	"github.com/pkg/errors"
)

func main() {
	config, err := config.New()
	if err != nil {
		panic(errors.Wrap(err, "config.New failed"))
	}

	log.Init(config)

	// Log the initial config info, for later debugging/checking/forensics
	/*
		startupLogFields := make(logrus.Fields)
		startupLogFields["configProvenances"] = config.Provenances()
		startupLogFields["config"] = config.Map()
		startupLogFields["version"] = config.Version()
		log.SystemLogNoCtx().WithFields(startupLogFields).Info("server started")
	*/

	db.Init(config)
	api.Init(config)

	// DEBUG
	fmt.Printf("%+v\n", config.Map())
	fmt.Printf("%+v\n", config.Provenances())
}
