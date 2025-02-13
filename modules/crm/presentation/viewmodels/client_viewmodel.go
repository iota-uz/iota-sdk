package viewmodels

import "strings"

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
	var result []string
	if c.FirstName != "" {
		result = append(result, c.FirstName)
	}
	if c.LastName != "" {
		result = append(result, c.LastName)
	}
	if len(result) == 0 {
		return "+" + c.Phone
	}
	return strings.Join(result, " ")
}

func (c *Client) Initials() string {
	res := ""
	if len(c.FirstName) > 0 {
		res += string(c.FirstName[0])
	}
	if len(c.LastName) > 0 {
		res += string(c.LastName[0])
	}
	if len(res) == 0 {
		return "NA"
	}
	return res
}
