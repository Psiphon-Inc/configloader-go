package log

// NOTE: No dependency on config!

// Configurer is the config interface required by this package
type Configurer interface {
	LogLevel() string
}

func Init(config Configurer) {
	// do stuff
}
