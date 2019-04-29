package api

import (
	// NOTE: Dependency on config!
	"github.com/Psiphon-Inc/configloader-go/examples/singletonfuncs/config"
)

func Init() {
	// Access the config singleton via package like so:
	_ = config.CORSUserAgentAllowed("fake UA")
}
