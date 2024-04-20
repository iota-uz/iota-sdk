package models

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"reflect"
)

func Insert(db *sqlx.DB, model Model) error {
	query := goqu.Insert(model.Table()).Rows(MapData(model))
	sql, _, err := query.ToSQL()
	if err != nil {
		return err
	}
	_, err = db.Exec(sql)
	return err
}

func Update(db *sqlx.DB, model Model) error {
	query := goqu.Update(model.Table()).Set(MapData(model)).Where(goqu.Ex{model.PkField().Name: model.Pk()})
	sql, _, err := query.ToSQL()
	if err != nil {
		return err
	}
	_, err = db.Exec(sql)
	return err
}

func Fields(model interface{}) []*Field {
	reflected := reflect.ValueOf(model).Elem()
	var fields []*Field
	for i := 0; i < reflected.NumField(); i++ {
		field := reflected.Type().Field(i)
		dbTag := field.Tag.Get("db")
		if dbTag == "" {
			continue
		}
		gqlTag := field.Tag.Get("gql")
		if gqlTag == "" || gqlTag == "-" {
			continue
		}
		belongTo := field.Tag.Get("belongs_to")
		if belongTo == "" {
			fields = append(fields, &Field{
				Name: dbTag,
				Type: reflectedTypeToDbType(field.Type),
			})
		} else {
			fields = append(fields, &Field{
				Name: dbTag,
				Type: reflectedTypeToDbType(field.Type),
				Association: &Association{
					To:     reflect.New(field.Type.Elem()).Interface().(Model),
					Column: belongTo,
					As:     gqlTag,
				},
			})
		}
	}
	return fields
}

func reflectedTypeToDbType(reflectedType reflect.Type) DataType {
	switch reflectedType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return Integer
	case reflect.Int64:
		return BigInt
	case reflect.Float32, reflect.Float64:
		return DoublePrecision
	case reflect.String:
		return Text
	case reflect.Bool:
		return Boolean
	case reflect.Ptr:
		return reflectedTypeToDbType(reflectedType.Elem())
	case reflect.Struct:
		if reflectedType.PkgPath() == "time" {
			if reflectedType.Name() == "Time" {
				return Timestamp
			}
		}
	default:
		panic(fmt.Sprintf("Unsupported type: %s", reflectedType.String()))
	}
	return Text
}

func Refs(model interface{}) []*Field {
	fields := Fields(model)
	var refs []*Field
	for _, field := range fields {
		if field.Association != nil {
			refs = append(refs, field)
		}
	}
	return refs
}

func MapData(model interface{}) map[string]interface{} {
	reflected := reflect.ValueOf(model).Elem()
	data := make(map[string]interface{})
	for i := 0; i < reflected.NumField(); i++ {
		field := reflected.Type().Field(i)
		tag := field.Tag.Get("db")
		if tag == "" {
			continue
		}
		f := reflected.Field(i)
		if f.Kind() == reflect.Ptr {
			if f.IsNil() {
				continue
			}
			f = f.Elem()
		}
		if f.IsZero() {
			continue
		}
		data[tag] = f.Interface()
	}
	return data
}
