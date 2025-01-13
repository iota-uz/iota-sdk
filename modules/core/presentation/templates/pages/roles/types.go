package roles

type Group struct {
	Label    string
	Children []*Child
}

type Child struct {
	Name    string
	Label   string
	Checked bool
}
