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
