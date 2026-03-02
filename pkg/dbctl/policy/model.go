package policy

type Config struct {
	Environments map[string]EnvironmentPolicy `yaml:"environments"`
	Credentials  CredentialPolicy             `yaml:"credentials"`
}

type EnvironmentPolicy struct {
	AllowedHosts     []string `yaml:"allowed_hosts"`
	AllowDestructive bool     `yaml:"allow_destructive"`
	RequireYes       bool     `yaml:"require_yes"`
	RequireTicket    bool     `yaml:"require_ticket"`
}

type CredentialPolicy struct {
	Emission       string `yaml:"emission"`
	TokenTTLSecond int    `yaml:"token_ttl_seconds"`
}

type Target struct {
	Environment string
	Host        string
	Port        string
	Name        string
	User        string
}

type Decision struct {
	Allowed            bool
	RequireYes         bool
	RequireTicket      bool
	AllowDestructive   bool
	CredentialEmission string
	Reasons            []string
}

func (d Decision) Denied(reason string) Decision {
	d.Allowed = false
	d.Reasons = append(d.Reasons, reason)
	return d
}
