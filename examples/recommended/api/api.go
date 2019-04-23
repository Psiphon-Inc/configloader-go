package api

import (
	// NOTE: No dependency on config!

	"github.com/Psiphon-Inc/configloader-go/examples/recommended/api/stats"
)

// Configurer is the config interface required by this package
type Configurer interface {
	// Implicitly require the Configurers for all packages that will be Init'd here
	stats.Configurer

	CORSUserAgentAllowed(ua string) bool
}

func Init(config Configurer) {
	stats.Init(config)

	// do stuff
}
