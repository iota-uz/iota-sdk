package application

type RuntimeProfile string

const (
	RuntimeProfileServer RuntimeProfile = "server"
	RuntimeProfileCLI    RuntimeProfile = "cli"
)
