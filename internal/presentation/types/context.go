package types

type PageContext struct {
	Title string
	Lang  string
}

func (p *PageContext) T(k string) string {
	return k
}
