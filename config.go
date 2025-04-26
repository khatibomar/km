package main

import (
	"encoding/json"
	"reflect"
)

type Settings struct {
	Override bool `toml:"override"`
}

type Source struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type Destination struct {
	Name          string            `toml:"name"`
	Path          string            `toml:"path"`
	IgnoredFields []string          `toml:"ignore"`
	FieldsMap     map[string]string `toml:"map"`
}

type Mapping struct {
	Settings     Settings      `toml:"settings"`
	Source       Source        `toml:"source"`
	Destinations []Destination `toml:"destination"`
	Plugins      []string      `toml:"plugins"`
}

type Config struct {
	Mappings []Mapping  `toml:"mappings"`
	Settings GenSetting `toml:"settings"`
}

func (c *Config) BuildTOMLSchema() string {
	schema := make(map[string]any)
	t := reflect.TypeOf(c).Elem()

	properties := make(map[string]any)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("toml")
		if tag == "" {
			continue
		}

		fieldSchema := make(map[string]any)
		switch field.Type.Kind() {
		case reflect.Struct:
			fieldSchema["type"] = "object"
			fieldSchema["properties"] = buildStructSchema(field.Type)
		case reflect.Slice:
			fieldSchema["type"] = "array"
			itemType := field.Type.Elem()
			if itemType.Kind() == reflect.Struct {
				fieldSchema["items"] = map[string]any{
					"type":       "object",
					"properties": buildStructSchema(itemType),
				}
			} else {
				fieldSchema["items"] = map[string]any{
					"type": getJSONSchemaType(itemType.Kind()),
				}
			}
		default:
			fieldSchema["type"] = getJSONSchemaType(field.Type.Kind())
		}
		properties[tag] = fieldSchema
	}

	schema["$schema"] = "http://json-schema.org/draft-07/schema#"
	schema["type"] = "object"
	schema["properties"] = properties
	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

func buildStructSchema(t reflect.Type) map[string]any {
	properties := make(map[string]any)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("toml")
		if tag == "" {
			continue
		}

		fieldSchema := make(map[string]any)
		switch field.Type.Kind() {
		case reflect.Struct:
			fieldSchema["type"] = "object"
			fieldSchema["properties"] = buildStructSchema(field.Type)
		case reflect.Slice:
			fieldSchema["type"] = "array"
			itemType := field.Type.Elem()
			if itemType.Kind() == reflect.Struct {
				fieldSchema["items"] = map[string]any{
					"type":       "object",
					"properties": buildStructSchema(itemType),
				}
			} else {
				fieldSchema["items"] = map[string]any{
					"type": getJSONSchemaType(itemType.Kind()),
				}
			}
		case reflect.Map:
			fieldSchema["type"] = "object"
			fieldSchema["additionalProperties"] = true
		default:
			fieldSchema["type"] = getJSONSchemaType(field.Type.Kind())
		}
		properties[tag] = fieldSchema
	}
	return properties
}

func getJSONSchemaType(kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return "string"
	}
}

type GenSetting struct {
	Style          string `toml:"style"`
	Module         string `toml:"module"`
	PathFromModule string `toml:"path_from_module"`
}
