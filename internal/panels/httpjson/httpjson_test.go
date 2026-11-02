package httpjson

import (
	"encoding/json"
	"testing"
)

func TestExtractPath(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(`{
		"data": {"items": [{"name": "alpha"}, {"name": "beta"}], "count": 2},
		"ok": true
	}`), &doc); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		expr string
		want any
		err  bool
	}{
		{"data.items[0].name", "alpha", false},
		{"data.items[1].name", "beta", false},
		{"data.count", float64(2), false},
		{"ok", true, false},
		{"data.items[9].name", nil, true},
		{"data.missing", nil, true},
		{"data.count.nested", nil, true},
	}
	for _, tc := range cases {
		got, err := ExtractPath(doc, tc.expr)
		if tc.err {
			if err == nil {
				t.Errorf("%s: expected error, got %v", tc.expr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.expr, got, tc.want)
		}
	}
}
