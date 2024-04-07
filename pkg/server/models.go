package server

import (
	"github.com/iota-agency/iota-erp/pkg/server/service"
)

var Models = []*service.Model{
	{
		Table: "users",
		Pk: &service.Field{
			Name: "id",
			Type: service.BigSerial,
		},
		Fields: []*service.Field{
			{
				Name:     "email",
				Type:     service.CharacterVarying,
				Nullable: false,
			},
		},
	},
	{
		Pk: &service.Field{
			Name: "id",
			Type: service.BigSerial,
		},
		Table: "companies",
		Fields: []*service.Field{
			{
				Name:     "name",
				Type:     service.CharacterVarying,
				Nullable: false,
			},
			{
				Name:     "address",
				Type:     service.CharacterVarying,
				Nullable: true,
			},
		},
	},
	{
		Pk: &service.Field{
			Name: "id",
			Type: service.BigSerial,
		},
		Table: "employees",
		Fields: []*service.Field{
			{
				Name:     "first_name",
				Type:     service.CharacterVarying,
				Nullable: false,
			},
			{
				Name:     "last_name",
				Type:     service.CharacterVarying,
				Nullable: false,
			},
			{
				Name:     "company_id",
				Type:     service.Integer,
				Nullable: false,
				Association: &service.Association{
					Table:  "companies",
					Column: "id",
					As:     "company",
				},
			},
			{
				Name:     "email",
				Type:     service.CharacterVarying,
				Nullable: false,
			},
			{
				Name:     "salary",
				Type:     service.Numeric,
				Nullable: false,
			},
			{
				Name:     "created_at",
				Type:     service.Timestamp,
				Nullable: false,
			},
			{
				Name:     "updated_at",
				Type:     service.Timestamp,
				Nullable: false,
			},
		},
	},
}
