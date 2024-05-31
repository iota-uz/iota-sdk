package adapters

import (
	"errors"
	"strings"
)

type GqlFieldMeta struct {
	Name      string
	Readable  bool
	Creatable bool
	Updatable bool
}

func GqlFieldMetaFromTag(tag string) (*GqlFieldMeta, error) {
	if tag == "" || tag == "-" {
		return nil, errors.New("invalid tag")
	}
	f := &GqlFieldMeta{
		Readable:  true,
		Creatable: true,
		Updatable: true,
	}
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return nil, errors.New("invalid tag")
	}
	if len(parts) == 1 {
		f.Name = parts[0]
		return f, nil
	}
	for _, v := range parts {
		switch v {
		case "!read":
			f.Readable = false
		case "!create":
			f.Creatable = false
		case "!update":
			f.Updatable = false
		}
	}
	f.Name = parts[0]
	return f, nil
}
