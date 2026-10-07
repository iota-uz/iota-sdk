package agentsignin

type Config struct {
	Enabled   bool   `koanf:"enabled"`
	Origin    string `koanf:"origin"`
	TenantID  string `koanf:"tenantid"`
	Directory string `koanf:"directory"`
}

func (Config) ConfigPrefix() string { return "agent.signin" }
