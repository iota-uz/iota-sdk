package employee

type language struct {
	primary   string
	secondary string
}

func NewLanguage(primary, secondary string) Language {
	return &language{
		primary:   primary,
		secondary: secondary,
	}
}

func (l *language) Primary() string {
	return l.primary
}

func (l *language) Secondary() string {
	return l.secondary
}
