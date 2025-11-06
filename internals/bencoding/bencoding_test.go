package bencoding

import (
	"reflect"
	"testing"
)

func TestParseBencode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
		valid    bool
	}{
		// ✅ Basic types
		{"Simple string", "4:spam", "spam", true},
		{"Empty string", "0:", "", true},
		{"Integer", "i42e", int64(42), true},
		{"Negative integer", "i-42e", int64(-42), true},
		{"Zero", "i0e", int64(0), true},

		// ✅ Lists
		{"Simple list", "l4:spam4:eggse", []any{"spam", "eggs"}, true},
		{"Empty list", "le", []any{}, true},
		{"Nested list", "ll4:spame4:eggse", []any{[]any{"spam"}, "eggs"}, true},

		// ✅ Dictionaries
		{"Simple dict", "d3:cow3:moo4:spam4:eggse", map[string]any{"cow": "moo", "spam": "eggs"}, true},
		{"Empty dict", "de", map[string]any{}, true},
		{"Nested dict", "d4:spaml1:a1:bee", map[string]any{"spam": []any{"a", "b"}}, true},
		{"Dict with int and list", "d3:bar4:spam3:fooi42ee", map[string]any{"bar": "spam", "foo": int64(42)}, true},

		// ⚠️ Edge and limit cases
		{"String with colon inside", "13:spam:eggs:ham", "spam:eggs:ham", true},
		{"Leading zero integer", "i042e", nil, false},
		{"Negative zero", "i-0e", nil, false},
		{"Missing 'e' in integer", "i42", nil, false},
		{"Unterminated list", "l4:spami42e", nil, false},
		{"Unterminated dict", "d3:cow3:moo", nil, false},
		{"Non-string key in dict", "di1e3:mooe", nil, false},
		{"Empty key in dict", "d0:3:mooe", map[string]any{"": "moo"}, true},

		// ✅ Deep nesting
		{"Deep nested lists", "llli1eeee", []any{[]any{[]any{int64(1)}}}, true},

		// ✅ Large integer
		{"Big integer", "i1234567890123456789e", int64(1234567890123456789), true},

		// ✅ Mixed list
		{"List with mixed types", "li42e4:spamdee", []any{int64(42), "spam", map[string]any{}}, true},

		// ⚠️ Invalid structure
		{"String length mismatch", "4:spa", nil, false},
		{"Whitespace not allowed", "i 42 e", nil, false},

		// ✅ Complex realistic sample (torrent-like)
		{
			"Torrent-like dict",
			"d8:announce14:http://tracker4:infod5:filesld6:lengthi12345e4:pathl8:file.txteeeee",
			map[string]any{
				"announce": "http://tracker",
				"info": map[string]any{
					"files": []any{
						map[string]any{
							"length": int64(12345),
							"path":   []any{"file.txt"},
						},
					},
				},
			},
			true,
		},
		// ✅ Recursive depth test
		{"Recursive depth", "lllllllllli1eeeeeeeeeeee", []any{[]any{[]any{[]any{[]any{[]any{[]any{[]any{[]any{[]any{int64(1)}}}}}}}}}}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := ParseBencode([]byte(tc.input))
			if tc.valid {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
				if !reflect.DeepEqual(out, tc.expected) {
					t.Fatalf("expected %#v, got %#v", tc.expected, out)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got none (output: %#v)", out)
				}
			}
		})
	}
}
