package client1c

type OdataService struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type OdataServices struct {
	OdataMetadata string         `json:"odata.metadata"`
	Value         []OdataService `json:"value"`
}
