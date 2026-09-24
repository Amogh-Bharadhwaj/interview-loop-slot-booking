package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Int64List backs a jsonb column holding a plain array of int64 IDs.
type Int64List []int64

func (l Int64List) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

func (l *Int64List) Scan(v any) error {
	if v == nil {
		*l = Int64List{}
		return nil
	}
	var b []byte
	switch t := v.(type) {
	case []byte:
		b = t
	case string:
		b = []byte(t)
	default:
		return fmt.Errorf("unsupported type for Int64List: %T", v)
	}
	if len(b) == 0 {
		*l = Int64List{}
		return nil
	}
	return json.Unmarshal(b, l)
}
