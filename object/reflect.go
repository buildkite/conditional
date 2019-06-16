package object

import (
	"reflect"
	"strings"
)

const (
	tagName = `conditional`
)

// NewReflectMap uses reflection to construct a Map from golang structs
// maps and native types
func NewReflectMap(i interface{}) Map {
	if iMap, ok := i.(map[string]interface{}); ok {
		return reflectInterfaceMapAsStruct(iMap)
	}
	return reflectStruct(i)
}

func reflectInterfaceMapAsStruct(i map[string]interface{}) *Struct {
	result := &Struct{}

	for k, v := range i {
		switch vi := v.(type) {
		case map[string]interface{}:
			result.Set(k, reflectInterfaceMapAsStruct(vi))
		case string:
			result.Set(k, &String{vi})
		case int:
			result.Set(k, &Integer{int64(vi)})
		case int64:
			result.Set(k, &Integer{vi})
		case bool:
			result.Set(k, &Boolean{vi})
		default:
			result.Set(k, reflectStruct(vi))
		}
	}

	return result
}

func reflectStruct(i interface{}) *Struct {
	t := reflect.TypeOf(i)

	// Only accept structs, this should probably error
	if t.Kind() != reflect.Struct {
		return nil
	}

	result := &Struct{}
	v := reflect.ValueOf(i)

	// Iterate over fields and use tags if we can
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get(tagName)

		// Use lowercase string if no tag
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}

		switch tv := v.Field(i).Interface().(type) {
		case map[string]interface{}:
			result.Set(tag, reflectInterfaceMapAsStruct(tv))
		case string:
			result.Set(tag, &String{tv})
		case int:
			result.Set(tag, &Integer{int64(tv)})
		case int64:
			result.Set(tag, &Integer{tv})
		case bool:
			result.Set(tag, &Boolean{tv})
		default:
			nestedResult := reflectStruct(tv)

			// Skip non-struct properties
			if nestedResult == nil {
				continue
			}
			result.Set(tag, nestedResult)
		}
	}

	return result
}
