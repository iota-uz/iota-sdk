package viewmodels

type Permission struct {
	ID       string
	Name     string
	Resource string
	Action   string
	Modifier string
}

func (p *Permission) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}

	return p.Action + " " + p.Resource + " (" + p.Modifier + ")"
}
