package viewmodels

type Client struct {
	ID         string
	FirstName  string
	LastName   string
	MiddleName string
	Phone      string
	CreatedAt  string
	UpdatedAt  string
}

func (c *Client) FullName() string {
	return c.FirstName + " " + c.LastName
}

func (c *Client) Initials() string {
	res := ""
	if len(c.FirstName) > 0 {
		res += string(c.FirstName[0])
	}
	if len(c.LastName) > 0 {
		res += string(c.LastName[0])
	}
	return res
}
