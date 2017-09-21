package shellexec

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		tokens []string
	}{
		{
			"all escape chars",
			`  echo  \|\&\;\<\>\(\)\$\\\"\'\ \	\` + "\n\\`",
			[]string{"echo", `|&;<>()$\"'` + " \t\n`"},
		},
		{
			"single-quote strings",
			`foo'bar''boo&;<>'`,
			[]string{`foobarboo&;<>`},
		},
		{
			"other special characters",
			`\*\?\[\#\~\=\%  =%`,
			[]string{"*?[#~=%", "=%"},
		},
	}

	for _, tt := range tests {
		p := parser{s: tt.line}
		got, err := p.parseLine()
		if err != nil {
			t.Errorf("%s: unexpected err: %v", tt.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.tokens) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.tokens)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		line string
		err  string
	}{
		{
			"bad esc seq",
			`date \e`,
			"unknown escape sequence",
		},
		{
			"unterminated single-quot string",
			`echo 'always'be'closin`,
			"string not terminated",
		},
	}

	for _, tt := range tests {
		p := parser{s: tt.line}
		_, err := p.parseLine()
		if err == nil {
			t.Errorf("%s: unexpectadly succeeded", tt.name)
			continue
		}
		if !strings.Contains(err.Error(), tt.err) {
			t.Errorf("%s: got %q, want %q", tt.name, err, tt.err)
		}
	}
}

func TestParseInvalidChars(t *testing.T) {
	invalid := []rune{'|', '&', ';', '<', '>', '(', ')', '$', '`', '"',
		'*', '?', '[', '#', '~'}

	for _, r := range invalid {
		p := parser{s: string(r)}
		if _, err := p.parseLine(); err == nil {
			t.Errorf("char %q should be invalid", r)
		}
	}
}
