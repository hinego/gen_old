package model

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// Column table column's info
type Column struct {
	gorm.ColumnType
	TableName   string                                       `gorm:"column:TABLE_NAME"`
	Indexes     []*Index                                     `gorm:"-"`
	UseScanType bool                                         `gorm:"-"`
	dataTypeMap map[string]func(c *Column) (dataType string) `gorm:"-"`
	jsonTagNS   func(columnName string) string               `gorm:"-"`
	newTagNS    func(columnName string) string               `gorm:"-"`
}

// SetDataTypeMap set data type map
func (c *Column) SetDataTypeMap(m map[string]func(col *Column) (dataType string)) {
	c.dataTypeMap = m
}

// GetDataType get data type
func (c *Column) GetDataType() (fieldtype string) {
	if mapping, ok := c.dataTypeMap[c.DatabaseTypeName()]; ok {
		return mapping(c)
	}
	if c.UseScanType && c.ScanType() != nil {
		return c.ScanType().String()
	}
	return dataType.Get(c.DatabaseTypeName(), c.columnType())
}

// WithNS with name strategy
func (c *Column) WithNS(jsonTagNS, newTagNS func(columnName string) string) {
	c.jsonTagNS, c.newTagNS = jsonTagNS, newTagNS
	if c.jsonTagNS == nil {
		c.jsonTagNS = func(n string) string { return n }
	}
	if c.newTagNS == nil {
		c.newTagNS = func(string) string { return "" }
	}
}

// ToField convert to field
func (c *Column) ToField(nullable, coverable, signable bool) *Field {
	fieldType := c.GetDataType()
	if signable && strings.Contains(c.columnType(), "unsigned") && strings.HasPrefix(fieldType, "int") {
		fieldType = "u" + fieldType
	}
	switch {
	case c.Name() == "deleted_at" && fieldType == "time.Time":
		fieldType = "gorm.DeletedAt"
	case coverable && c.needDefaultTag(c.defaultTagValue()):
		fieldType = "*" + fieldType
	case nullable:
		if n, ok := c.Nullable(); ok && n {
			fieldType = "*" + fieldType
		}
	}

	var comment string
	if c, ok := c.Comment(); ok {
		comment = c
	}
	//marshal, _ := json.Marshal(&Field{
	//	Name:             c.Name(),
	//	Type:             fieldType,
	//	ColumnName:       c.Name(),
	//	MultilineComment: c.multilineComment(),
	//	GORMTag:          c.buildGormTag(),
	//	JSONTag:          c.jsonTagNS(c.Name()),
	//	NewTag:           c.newTagNS(c.Name()),
	//	ColumnComment:    comment,
	//})
	var field string
	if strings.Contains(comment, "|") {
		arr := strings.Split(comment, "|")
		comment = arr[1]
		if len(arr) >= 3 {
			fieldType = arr[1]
			comment = arr[2]
		}
		if len(arr) >= 4 {
			field = arr[3]
		}
	}

	return &Field{
		Name:             c.Name(),
		Type:             fieldType,
		Field:            field,
		ColumnName:       c.Name(),
		MultilineComment: c.multilineComment(),
		GORMTag:          c.buildGormTag(),
		JSONTag:          c.jsonTagNS(c.Name()),
		NewTag:           c.newTagNS(c.Name()),
		ColumnComment:    comment,
	}
}

func (c *Column) multilineComment() bool {
	cm, ok := c.Comment()
	return ok && strings.Contains(cm, "\n")
}

func (c *Column) buildGormTag() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("column:%s;type:%s", c.Name(), c.columnType()))
	//if c.columnType() == "json" {
	//	buf.WriteString(";serializer:json")
	//}
	if d, ok := c.Comment(); ok {
		if strings.Contains(d, "|") {
			arr := strings.Split(d, "|")
			if len(arr) > 2 && arr[0] != "" {
				buf.WriteString(";serializer:" + arr[0])
			}
		}
	}
	isPriKey, ok := c.PrimaryKey()
	isValidPriKey := ok && isPriKey
	if isValidPriKey {
		buf.WriteString(";primaryKey")
		if at, ok := c.AutoIncrement(); ok {
			buf.WriteString(fmt.Sprintf(";autoIncrement:%t", at))
		}
	} else if n, ok := c.Nullable(); ok && !n {
		buf.WriteString(";not null")
	}

	for _, idx := range c.Indexes {
		if idx == nil {
			continue
		}
		if pk, _ := idx.PrimaryKey(); pk { //ignore PrimaryKey
			continue
		}
		if uniq, _ := idx.Unique(); uniq {
			buf.WriteString(fmt.Sprintf(";uniqueIndex:%s,priority:%d", idx.Name(), idx.Priority))
		} else {
			buf.WriteString(fmt.Sprintf(";index:%s,priority:%d", idx.Name(), idx.Priority))
		}
	}

	if dtValue := c.defaultTagValue(); !isValidPriKey && c.needDefaultTag(dtValue) { // cannot set default tag for primary key
		buf.WriteString(fmt.Sprintf(`;default:%s`, dtValue))
	}
	return buf.String()
}

// needDefaultTag check if default tag needed
func (c *Column) needDefaultTag(defaultTagValue string) bool {
	if defaultTagValue == "" {
		return false
	}
	switch c.ScanType().Kind() {
	case reflect.Bool:
		return defaultTagValue != "false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return defaultTagValue != "0"
	case reflect.String:
		return defaultTagValue != ""
	case reflect.Struct:
		return strings.Trim(defaultTagValue, "'0:- ") != "" && strings.ToUpper(strings.TrimSpace(defaultTagValue)) != "CURRENT_TIMESTAMP"
	}
	return c.Name() != "created_at" && c.Name() != "updated_at"
}

// defaultTagValue return gorm default tag's value
func (c *Column) defaultTagValue() string {
	value, ok := c.DefaultValue()
	if !ok {
		return ""
	}
	if value != "" && strings.TrimSpace(value) == "" {
		return "'" + value + "'"
	}
	return value
}

func (c *Column) columnType() (v string) {
	if cl, ok := c.ColumnType.ColumnType(); ok {
		return cl
	}
	return c.DatabaseTypeName()
}
