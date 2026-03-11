package policy

type Config struct {
	Environments map[string]EnvironmentPolicy `yaml:"environments"`
}

type EnvironmentPolicy struct {
	AllowedHosts     []string `yaml:"allowed_hosts"`
	AllowDestructive bool     `yaml:"allow_destructive"`
	RequireYes       bool     `yaml:"require_yes"`
	RequireTicket    bool     `yaml:"require_ticket"`
}

type Target struct {
	Environment string
	Host        string
	Port        string
	Name        string
	User        string
}

type Decision struct {
	Allowed          bool
	RequireYes       bool
	RequireTicket    bool
	AllowDestructive bool
	Reasons          []string
}

func (d Decision) Denied(reason string) Decision {
	d.Allowed = false
	d.Reasons = append(d.Reasons, reason)
	return d
}
