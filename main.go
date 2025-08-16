package main

import (
	"fmt"
	"go/format"
	"reflect"
	"strings"
	"unicode"

	"github.com/khatibomar/km/internal/user"
	userdto "github.com/khatibomar/km/internal/userDTO"
)

func incompatibleTypePanic(source, destination reflect.Type) {
	sb := strings.Builder{}
	sb.Grow(512)
	sb.WriteString("can not map ")
	if source.Name() != "" {
		sb.WriteString(source.Name() + " of ")
	}
	sb.WriteString("type " + source.Kind().String() + " to ")
	if destination.Name() != "" {
		sb.WriteString(destination.Name() + " of ")
	}
	sb.WriteString("type " + destination.Kind().String())
	panic(sb.String())
}

func receiverBuilder(in string) string {
	if in == "" {
		return "x"
	}

	runes := []rune(in)
	var result []rune

	for i, c := range runes {
		if unicode.IsUpper(c) {
			if i == 0 || !unicode.IsUpper(runes[i-1]) {
				result = append(result, unicode.ToLower(c))
			}
		} else if unicode.IsLetter(c) && i == 0 {
			result = append(result, unicode.ToLower(c))
		}
	}

	return string(result)
}

func Map[S any, D any]() error {
	var source S
	var destination D

	srcType := reflect.TypeOf(source)
	dstType := reflect.TypeOf(destination)

	if srcType.Kind() != dstType.Kind() {
		if srcType.Kind() == reflect.Struct && (dstType.Kind() == reflect.Slice || dstType.Kind() == reflect.Array) {
			incompatibleTypePanic(srcType, dstType)
		}
	}

	fmt.Println(srcType.PkgPath())
	fmt.Println(dstType.PkgPath())

	var code strings.Builder
	code.Grow(1024)

	recv := receiverBuilder(srcType.Name())

	switch dstType.Kind() {
	case reflect.Struct:
		fmt.Fprintf(&code, "func (%s *%s) To%s() %s {\n", recv, srcType.Name(), dstType.Name(), dstType.Name())
		fmt.Fprintf(&code, "r := %s{}\n", dstType.Name())
		for i := 0; i < srcType.NumField(); i++ {
			field := srcType.Field(i)
			if _, ok := dstType.FieldByName(field.Name); ok {
				fmt.Fprintf(&code, "r.%s = %s.%s\n", field.Name, recv, field.Name)
			}
		}
		code.WriteString("return r\n}\n")
	case reflect.Map:
		fmt.Fprintf(&code, "func (%s *%s) ToMap() map[string]any {\n", recv, srcType.Name())
		fmt.Fprintf(&code, "r := make(map[string]any, %d)\n", srcType.NumField())
		for i := 0; i < srcType.NumField(); i++ {
			field := srcType.Field(i)
			fmt.Fprintf(&code, "r[\"%s\"] = %s.%s\n", field.Name, recv, field.Name)
		}
		code.WriteString("return r\n}\n")
	}

	formatted, err := format.Source([]byte(code.String()))
	if err != nil {
		return fmt.Errorf("format error: %w", err)
	}

	fmt.Println(string(formatted))
	return nil
}

func main() {
	Map[user.User, userdto.UserDTO]()
	Map[userdto.UserDTO, map[string]any]()
	// Map(u, []int{3})
}
