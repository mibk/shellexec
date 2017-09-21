package shellexec

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrUnknownEscSeq      = errors.New("unknown escape sequence")
	ErrUnterminatedString = errors.New("string not terminated")
)

// Command parses line as a pseudo-shell command and returns
// the os/exec.Cmd struct to execute the line.
func Command(line string) (*exec.Cmd, error) {
	s := parser{s: line}
	fs, err := s.parseLine()
	if err != nil {
		return nil, err
	}
	envIndex := 0
	for _, f := range fs {
		if !strings.ContainsRune(f, '=') {
			break
		}
		envIndex++
	}
	env := append(os.Environ(), fs[:envIndex]...)
	fs = fs[envIndex:]

	if len(fs) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := exec.Command(fs[0], fs[1:]...)
	cmd.Env = env
	return cmd, nil
}

type parser struct {
	buf bytes.Buffer
	s   string
}

func (p *parser) parseLine() (fields []string, err error) {
	for len(p.s) > 0 {
		r, size := utf8.DecodeRuneInString(p.s)
		if !unicode.IsSpace(r) {
			f, err := p.parseField()
			if err != nil {
				return nil, err
			}
			fields = append(fields, f)
			continue
		}
		p.s = p.s[size:]
	}
	return fields, nil
}

func (p *parser) parseField() (string, error) {
	p.buf.Reset()
	esc := false
	for len(p.s) > 0 {
		r, size := utf8.DecodeRuneInString(p.s)
		p.s = p.s[size:]

		if esc {
			switch r {
			case '|', '&', ';', '<', '>', '(', ')', '$',
				'`', '\\', '"', '\'', ' ', '\t', '\n',
				'*', '?', '[', '#', '~', '=', '%':
				p.buf.WriteRune(r)
			default:
				return "", ErrUnknownEscSeq
			}
			esc = false
			continue
		}
		if unicode.IsSpace(r) {
			break
		}
		switch r {
		case '\'':
			s, err := p.parseSingleQuotes()
			if err != nil {
				return "", err
			}
			p.buf.WriteString(s)
		case '\\':
			esc = true
			continue
		case '|', '&', ';', '<', '>', '(', ')', '$', '`', '"',
			// Forbid these characters as they may need to be
			// quoted under certain circumstances.
			'*', '?', '[', '#', '~':
			return "", errors.New("unsupported character: " + string(r))
		default:
			p.buf.WriteRune(r)
		}
	}

	return p.buf.String(), nil
}

func (p *parser) parseSingleQuotes() (string, error) {
	for i, r := range p.s {
		if r == '\'' {
			str := p.s[:i]
			p.s = p.s[i+1:]
			return str, nil
		}
	}
	return "", ErrUnterminatedString
}
