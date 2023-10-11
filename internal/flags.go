package internal

// Flags define CLI flags.
type Flags struct {
	// Config is the path to the config file
	Config string `short:"c" long:"config" description:"path to config file" required:"true" default:"./config.yml"`
}
