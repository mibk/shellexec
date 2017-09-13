package shellexec

import (
	"reflect"
	"testing"
)

func TestScan(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		tokens []string
	}{
		{
			"all escape chars",
			`  echo  \|\&\;\<\>\(\)\$\\\"\'\ \	\` + "\n\\`",
			[]string{"echo", `|&;<>()$\"'` + " \t\n`"},
		}, {
			"single-quote strings",
			`foo'bar''boo&;<>'`,
			[]string{`foobarboo&;<>`},
		},
	}

	for _, tt := range tests {
		got, _ := scan(tt.line)
		if !reflect.DeepEqual(got, tt.tokens) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.tokens)
		}
	}
}

func TestScanInvalidChars(t *testing.T) {
	invalid := []rune{'|', '&', ';', '<', '>', '(', ')', '$', '`', '"'}
	for _, r := range invalid {
		if _, err := scan(string(r)); err == nil {
			t.Errorf("char %q should be invalid", r)
		}
	}
}
