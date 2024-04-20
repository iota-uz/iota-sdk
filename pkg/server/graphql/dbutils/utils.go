package dbutils

import (
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"strings"
)

func nestMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		parts := strings.Split(k, ".")
		lastKey := parts[len(parts)-1]
		m := result
		for _, part := range parts[:len(parts)-1] {
			if _, ok := m[part]; !ok {
				m[part] = make(map[string]interface{})
			}
			m = m[part].(map[string]interface{})
		}
		m[lastKey] = v
	}
	return result
}

func IsNumeric(kind models.DataType) bool {
	numerics := []models.DataType{
		models.Integer,
		models.BigSerial,
		models.SmallSerial,
		models.Serial,
		models.Numeric,
		models.Real,
		models.DoublePrecision,
	}
	return utils.Includes(numerics, kind)
}

func IsString(kind models.DataType) bool {
	vals := []models.DataType{
		models.Character,
		models.CharacterVarying,
		models.Text,
	}
	return utils.Includes(vals, kind)
}

func IsTime(kind models.DataType) bool {
	times := []models.DataType{
		models.Date,
		models.Time,
		models.Timestamp,
	}
	return utils.Includes(times, kind)
}
