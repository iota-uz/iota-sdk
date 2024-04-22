package models

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"reflect"
)

type dataTypeWithMeta struct {
	dataType DataType
	nullable bool
}

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
		t := reflectedTypeToDbType(field.Type)
		if belongTo == "" {
			fields = append(fields, &Field{
				Name:     dbTag,
				Type:     t.dataType,
				Nullable: t.nullable,
			})
		} else {
			fields = append(fields, &Field{
				Name:     dbTag,
				Type:     t.dataType,
				Nullable: t.nullable,
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

func reflectedTypeToDbType(reflectedType reflect.Type) *dataTypeWithMeta {
	switch reflectedType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return &dataTypeWithMeta{dataType: Integer, nullable: false}
	case reflect.Int64:
		return &dataTypeWithMeta{dataType: BigInt, nullable: false}
	case reflect.Float32, reflect.Float64:
		return &dataTypeWithMeta{dataType: DoublePrecision, nullable: false}
	case reflect.String:
		return &dataTypeWithMeta{dataType: Text, nullable: false}
	case reflect.Bool:
		return &dataTypeWithMeta{dataType: Boolean, nullable: false}
	case reflect.Ptr:
		if reflectedType.Implements(reflect.TypeOf((*Model)(nil)).Elem()) {
			return &dataTypeWithMeta{dataType: BigInt, nullable: false}
		}
		return reflectedTypeToDbType(reflectedType.Elem())
	case reflect.Struct:
		if reflectedType.PkgPath() == "time" && reflectedType.Name() == "Time" {
			return &dataTypeWithMeta{dataType: Timestamp, nullable: false}
		}
		switch reflectedType.Name() {
		case "JsonNullString":
			return &dataTypeWithMeta{dataType: CharacterVarying, nullable: true}
		case "JsonNullInt64":
			return &dataTypeWithMeta{dataType: BigInt, nullable: true}
		case "JsonNullInt32":
			return &dataTypeWithMeta{dataType: Integer, nullable: true}
		case "JsonNullFloat32":
			return &dataTypeWithMeta{dataType: Real, nullable: true}
		case "JsonNullFloat64":
			return &dataTypeWithMeta{dataType: DoublePrecision, nullable: true}
		case "JsonNullBool":
			return &dataTypeWithMeta{dataType: Boolean, nullable: true}
		}
	default:
		panic(fmt.Sprintf("Unsupported type: %s", reflectedType.String()))
	}
	panic(fmt.Sprintf("Unsupported type: %s", reflectedType.String()))
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
