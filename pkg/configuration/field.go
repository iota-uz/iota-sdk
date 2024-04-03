package configuration

type Field struct {
	Name     string `json:"name"`
	ReadOnly bool   `json:"read_only"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
}
