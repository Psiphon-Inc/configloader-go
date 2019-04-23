package stats

// NOTE: No dependency on config!

// Configurer is the config interface required by this package
type Configurer interface {
	StatsSampleCount() int
}

func Init(config Configurer) {
	// do stuff
}
