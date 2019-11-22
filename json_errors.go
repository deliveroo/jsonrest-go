package jsonrest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// jsonErrorDetails returns a "safe" error message indicating the cause of the
// JSON unmarshal error in terms that are safe to return to the caller.
func jsonErrorDetails(err error) string {
	switch err := err.(type) {
	case *json.SyntaxError:
		return fmt.Sprintf("offset %d: %s", err.Offset, err.Error())
	case *json.UnmarshalTypeError:
		var typeSuffix string
		if t := jsonType(err.Type); t != "" {
			typeSuffix = " (expected " + t + ")"
		}
		return fmt.Sprintf("offset %d: cannot unmarshal %s to %q%s", err.Offset, err.Value, err.Field, typeSuffix)
	default:
		return ""
	}
}

// jsonType attempts to map the given Go type to its equivalent JSON type. Note
// that this mapping is incomplete for custom types, since it's impossible to
// know what a custom UnmarshalJSON implementation may be doing.
func jsonType(t reflect.Type) (result string) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// Primitive types:
	if t.PkgPath() == "" {
		switch t.Kind() {
		case reflect.Bool:
			return "boolean"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return "integer"
		case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
			return "number"
		case reflect.String:
			return "string"
		}
	}

	// Common stdlib types:
	switch t {
	case typeTimeTime:
		return "time"
	case typeTimeDuration:
		return "duration"
	}

	// Structural types:
	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.Array, reflect.Slice:
		return "array"
	}

	return "" // unknown
}

var (
	typeTimeTime     = reflect.TypeOf((*time.Time)(nil)).Elem()
	typeTimeDuration = reflect.TypeOf((*time.Duration)(nil)).Elem()
)
