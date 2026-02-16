// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

import (
	"encoding/json"
	"reflect"
	"strings"
)

// MarshalJSON implements custom JSON marshaling for the Config struct.
// It uses reflection to traverse the struct and generate a map using "hcl" struct tags
// as the keys. This allows "hcl" tags to be the single source of truth for both
// HCL file I/O and JSON API output.
func (c *Config) MarshalJSON() ([]byte, error) {
	// Convert the config struct to a map using HCL tags
	data, err := toJSONMap(reflect.ValueOf(c))
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

// toJSONMap recursively converts a value to a map/slice/primitive for JSON marshaling,
// using "hcl" tags for struct field names.
func toJSONMap(v reflect.Value) (interface{}, error) {
	if !v.IsValid() {
		return nil, nil
	}

	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}

	// Handle SecureString explicitly
	if v.Type() == reflect.TypeOf(SecureString("")) {
		if v.String() == "" {
			return "", nil
		}
		return "(hidden)", nil
	}

	switch v.Kind() {
	case reflect.Struct:
		out := make(map[string]interface{})
		t := v.Type()

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			val := v.Field(i)

			// Get HCL tag
			tag := field.Tag.Get("hcl")
			if tag == "" {
				continue
			}

			// Parse tag parts
			parts := strings.Split(tag, ",")
			name := parts[0]

			if name == "-" {
				continue
			}

			// Handle optional/omitempty logic
			isOptional := false
			isBlock := false

			for _, p := range parts[1:] {
				if p == "optional" {
					isOptional = true
				}
				if p == "block" {
					isBlock = true
				}
			}

			// Recursively process the value
			jsonVal, err := toJSONMap(val)
			if err != nil {
				return nil, err
			}

			// Logic for omitting fields:
			// 1. If jsonVal is nil, it often means empty/nil pointer.
			// 2. We omit if optional or block AND jsonVal is nil OR isEmpty.
			// However, if we are inside a slice, we shouldn't return nil for the *element*
			// just because fields are empty, but here we are processing a field of a struct.

			if jsonVal == nil {
				if isOptional || isBlock {
					continue
				}
			} else {
				if isOptional || isBlock {
					if isEmpty(val) {
						continue
					}
				}
			}

			out[name] = jsonVal
		}
		// IMPORTANT: Even if the map is empty, we must return the empty map
		// so that a slice of structs doesn't contain nil elements.
		return out, nil

	case reflect.Slice, reflect.Array:
		out := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			val, err := toJSONMap(v.Index(i))
			if err != nil {
				return nil, err
			}
			out[i] = val
		}
		return out, nil

	case reflect.Map:
		out := make(map[string]interface{})
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String() // Assume string keys for map
			val, err := toJSONMap(iter.Value())
			if err != nil {
				return nil, err
			}
			out[k] = val
		}
		return out, nil

	default:
		return v.Interface(), nil
	}
}

func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
