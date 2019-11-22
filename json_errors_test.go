package jsonrest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestJSONErrorDetails(t *testing.T) {
	var dest struct {
		Name    string   `json:"name"`
		Count   int      `json:"count"`
		Enabled bool     `json:"enabled"`
		Values  []string `json:"values"`
		Nested  struct {
			Count int `json:"count"`
		} `json:"nested"`
	}
	tests := []struct {
		json string
		err  string
	}{
		{
			json: `{`,
			err:  `offset 1: unexpected end of JSON input`,
		},
		{
			json: `{"count": "abc"}`,
			err:  `offset 15: cannot unmarshal string to "count" (expected integer)`,
		},
		{
			json: `{"name": 1}`,
			err:  `offset 10: cannot unmarshal number to "name" (expected string)`,
		},
		{
			json: `{"name": false}`,
			err:  `offset 14: cannot unmarshal bool to "name" (expected string)`,
		},
		{
			json: `{"nested": {"count": "abc"}}`,
			err:  `offset 26: cannot unmarshal string to "nested.count" (expected integer)`,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			err := json.Unmarshal([]byte(tt.json), &dest)
			if err == nil {
				t.Fatal("unexpected nil error")
			}

			got := jsonErrorDetails(err)
			if got != tt.err {
				t.Errorf("incorrect error:\ngot:  %s\nwant: %s", got, tt.err)
			}
		})
	}
}

func TestJSONType(t *testing.T) {
	type Enum int
	type Person struct{}
	tests := []struct {
		val  interface{}
		want string
	}{
		{1, "integer"},
		{1.2, "number"},
		{[]string{}, "array"},
		{[...]string{}, "array"},
		{time.Now(), "time"},
		{time.Second, "duration"},
		{Enum(1), ""},
		{map[string]string{}, "object"},
		{Person{}, "object"},
		{&Person{}, "object"},
	}

	for _, tt := range tests {
		var (
			theType = reflect.TypeOf(tt.val)
			got     = jsonType(theType)
		)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", theType, got, tt.want)
		}
	}
}
