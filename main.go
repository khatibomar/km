package main

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"go/format"
	"reflect"
	"strings"
	"unicode"

	"github.com/khatibomar/km/internal/user"
	userdto "github.com/khatibomar/km/internal/userDTO"
)

func makeShortHash(input string) string {
	h := sha256.Sum256([]byte(input))
	encoded := base32.StdEncoding.EncodeToString(h[:])
	encoded = strings.ToLower(strings.TrimRight(encoded, "="))

	if encoded[0] >= '0' && encoded[0] <= '9' {
		encoded = "x" + encoded
	}

	return encoded[:12]
}

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

func Map[S any, D any]() {
	var source S
	var destination D

	srcType := reflect.TypeOf(source)
	dstType := reflect.TypeOf(destination)

	if srcType.Kind() != dstType.Kind() {
		if srcType.Kind() == reflect.Struct && (dstType.Kind() == reflect.Slice || dstType.Kind() == reflect.Array) {
			incompatibleTypePanic(srcType, dstType)
		}
	}

	srcPkg := srcType.PkgPath()
	srcHash := makeShortHash(srcPkg)
	dstPkg := dstType.PkgPath()
	dstHash := makeShortHash(dstPkg)

	var code strings.Builder
	code.Grow(1024)
	fmt.Fprintf(&code, "package km\n")
	fmt.Fprintf(&code, "import (\n")
	if srcPkg != "" {
		fmt.Fprintf(&code, "%s \"%s\"\n", srcHash, srcPkg)
	}
	if dstPkg != "" {
		fmt.Fprintf(&code, "%s \"%s\"\n", dstHash, dstPkg)
	}
	fmt.Fprintf(&code, ")\n")

	recv := receiverBuilder(srcType.Name())

	switch dstType.Kind() {
	case reflect.Struct:
		fmt.Fprintf(&code, "func %sTo%s(%s %s.%s) %s {\n",
			srcType.Name(),
			dstType.Name(),
			recv,
			srcHash,
			srcType.Name(),
			dstType.Name())
		fmt.Fprintf(&code, "r := %s.%s{}\n", dstHash, dstType.Name())
		for i := 0; i < srcType.NumField(); i++ {
			field := srcType.Field(i)
			if _, ok := dstType.FieldByName(field.Name); ok {
				fmt.Fprintf(&code, "r.%s = %s.%s\n", field.Name, recv, field.Name)
			}
		}
		code.WriteString("return r\n}\n")
	case reflect.Map:
		fmt.Fprintf(&code, "func %sToMap(%s %s.%s) map[string]any {\n", srcType.Name(), recv, srcHash, srcType.Name())
		fmt.Fprintf(&code, "r := make(map[string]any, %d)\n", srcType.NumField())
		for i := 0; i < srcType.NumField(); i++ {
			field := srcType.Field(i)
			fmt.Fprintf(&code, "r[\"%s\"] = %s.%s\n", field.Name, recv, field.Name)
		}
		code.WriteString("return r\n}\n")
	}

	fmt.Printf("======== DEBUG ========: %s\n================\n", string(code.String()))

	formatted, err := format.Source([]byte(code.String()))
	if err != nil {
		panic(fmt.Errorf("format error: %w", err))
	}

	fmt.Println(string(formatted))
}

func main() {
	Map[user.User, userdto.UserDTO]()
	Map[userdto.UserDTO, map[string]any]()
	// Map(u, []int{3})
}
