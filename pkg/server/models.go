package server

import (
	"github.com/iota-agency/iota-erp/pkg/server/graphql/service"
)

var UploadsModel = &service.Model{
	Pk: &service.Field{
		Name: "id",
		Type: service.BigSerial,
	},
	Table: "uploads",
	Fields: []*service.Field{
		{
			Name: "name",
			Type: service.CharacterVarying,
		},
		{
			Name: "path",
			Type: service.CharacterVarying,
		},
		{
			Name: "uploader_id",
			Type: service.Integer,
		},
		{
			Name: "mimetype",
			Type: service.CharacterVarying,
		},
		{
			Name: "size",
			Type: service.Real,
		},
		{
			Name: "created_at",
			Type: service.Timestamp,
		},
		{
			Name: "updated_at",
			Type: service.Timestamp,
		},
	},
}

var CompaniesModel = &service.Model{
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
		{
			Name:     "logo_id",
			Type:     service.Integer,
			Nullable: true,
			Association: &service.Association{
				To:     UploadsModel,
				Column: "id",
				As:     "logo",
			},
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
}

var PositionsModel = &service.Model{
	Pk: &service.Field{
		Name: "id",
		Type: service.BigSerial,
	},
	Table: "positions",
	Fields: []*service.Field{
		{
			Name: "name",
			Type: service.CharacterVarying,
		},
		{
			Name: "description",
			Type: service.Text,
		},
		{
			Name: "created_at",
			Type: service.Timestamp,
		},
		{
			Name: "updated_at",
			Type: service.Timestamp,
		},
	},
}

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
	CompaniesModel,
	PositionsModel,
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
				Name:     "phone",
				Type:     service.CharacterVarying,
				Nullable: false,
			},
			{
				Name:     "position_id",
				Type:     service.Integer,
				Nullable: false,
				Association: &service.Association{
					To:     PositionsModel,
					Column: "id",
				},
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
