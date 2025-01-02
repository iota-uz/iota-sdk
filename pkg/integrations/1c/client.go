package client1c

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Println(err)
		}
	}()

	var odataServices OdataServices
	if err := json.NewDecoder(res.Body).Decode(&odataServices); err != nil {
		return nil, err
	}
	return &odataServices, nil
}
