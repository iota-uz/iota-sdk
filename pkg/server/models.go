package server

import (
	"github.com/iota-agency/iota-erp/pkg/server/service"
)

//(
//id          SERIAL PRIMARY KEY,
//name        VARCHAR(255) NOT NULL,
//path        VARCHAR(255) NOT NULL,
//uploader_id INT          REFERENCES users (id) ON DELETE SET NULL,
//mimetype    VARCHAR(255) NOT NULL,
//size        FLOAT        NOT NULL,
//created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
//updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
//);

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
					To:     CompaniesModel,
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
