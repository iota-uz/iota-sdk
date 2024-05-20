package adapters

import (
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"gorm.io/gorm/schema"
	"reflect"
)

func ReadTypeFromModel(model interface{}, name string) (*graphql.Object, error) {
	gormFields, err := GetGormFields(model, func(f *schema.Field, gf *GqlFieldMeta) bool {
		return f.Readable && gf.Readable
	})
	if err != nil {
		return nil, err
	}
	gqlFields := graphql.Fields{}
	for key, field := range gormFields {
		if field.DataType == "" {
			refName := fmt.Sprintf("%s%sJoin", name, key)
			obj, err := ReadTypeFromModel(reflect.New(field.FieldType).Elem().Interface(), refName)
			if err != nil {
				return nil, err
			}
			gqlFields[key] = &graphql.Field{
				Type: obj,
			}
		} else {
			gqlFields[key] = GormFieldToGraphqlField(field)
		}
	}
	return graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: gqlFields,
	}), nil
}

func CreateArgsFromModel(model interface{}) (graphql.InputObjectConfigFieldMap, error) {
	gormFields, err := GetGormFields(model, func(f *schema.Field, gf *GqlFieldMeta) bool {
		return f.Creatable && gf.Creatable
	})
	if err != nil {
		return nil, nil
	}
	args := graphql.InputObjectConfigFieldMap{}
	for key, field := range gormFields {
		if field.DataType == "" {
			obj, err := CreateArgsFromModel(reflect.New(field.FieldType).Elem().Interface())
			if err != nil {
				return nil, err
			}
			args[key] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   fmt.Sprintf("%sCreateInput", utils.Title(key)),
					Fields: obj,
				}),
			}
		} else {
			args[key] = GormFieldToGraphqlInputField(field)
		}
	}
	return args, nil
}

func UpdateArgsFromModel(model interface{}) (graphql.InputObjectConfigFieldMap, error) {
	gormFields, err := GetGormFields(model, func(f *schema.Field, gf *GqlFieldMeta) bool {
		return f.Updatable && gf.Updatable
	})
	if err != nil {
		return nil, nil
	}
	args := graphql.InputObjectConfigFieldMap{}
	for key, field := range gormFields {
		if field.DataType == "" {
			obj, err := CreateArgsFromModel(reflect.New(field.FieldType).Elem().Interface())
			if err != nil {
				return nil, err
			}
			args[key] = &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   fmt.Sprintf("%sUpdateInput", utils.Title(key)),
					Fields: obj,
				}),
			}
		} else {
			args[key] = GormFieldToGraphqlInputField(field)
		}
	}
	return args, nil
}
