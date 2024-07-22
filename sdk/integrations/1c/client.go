package client_1c

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func NewClient(url string) *Client {
	return &Client{
		client: &http.Client{},
		url:    url,
	}
}

type Client struct {
	client *http.Client
	url    string
}

func (c *Client) GetOdataServices(infoBase string) (*OdataServices, error) {
	url := c.url + fmt.Sprintf("/%s/odata/standard.odata?$format=json", infoBase)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var odataServices OdataServices
	if err := json.NewDecoder(res.Body).Decode(&odataServices); err != nil {
		return nil, err
	}
	return &odataServices, nil
}
