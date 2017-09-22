package shellexec

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrEmptyCommand       = errors.New("empty command")
	ErrUnknownEscSeq      = errors.New("unknown escape sequence")
	ErrUnterminatedString = errors.New("string not terminated")
)

// Command parses line as a pseudo-shell command and returns
// the os/exec.Cmd struct to execute the line.
func Command(line string) (*exec.Cmd, error) {
	s := parser{s: line}

	c, err := s.parseLine()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(c.cmd, c.args...)
	cmd.Env = c.env
	return cmd, nil
}

type cmd struct {
	cmd  string
	args []string
	env  []string
}

type parser struct {
	buf bytes.Buffer
	s   string
}

func (p *parser) parseLine() (cmd, error) {
	var c cmd
	for len(p.s) > 0 {
		r, size := utf8.DecodeRuneInString(p.s)
		if unicode.IsSpace(r) {
			p.s = p.s[size:]
			continue
		}

		f, err := p.parseField()
		if err != nil {
			return cmd{}, err
		}

		if c.cmd == "" {
			if strings.ContainsRune(f, '=') {
				c.env = append(c.env, f)
			} else {
				c.cmd = f
			}
		} else {
			c.args = append(c.args, f)
		}
	}
	if c.cmd == "" {
		return cmd{}, ErrEmptyCommand
	}
	return c, nil
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
		case '"':
			s, err := p.parseDoubleQuotes()
			if err != nil {
				return "", err
			}
			p.buf.WriteString(s)
		case '\\':
			esc = true
			continue
		case '|', '&', ';', '<', '>', '(', ')', '$', '`',
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

func (p *parser) parseDoubleQuotes() (string, error) {
	var buf bytes.Buffer
	var esc bool
	for i, r := range p.s {
		if esc {
			switch r {
			default:
				buf.WriteRune('\\')
				fallthrough
			case '$', '`', '"', '\\':
				buf.WriteRune(r)
			case '\n':
				// Do nothing.
			}
			esc = false
			continue
		}
		switch r {
		case '"':
			p.s = p.s[i+1:]
			return buf.String(), nil
		case '\\':
			esc = true
			continue
		case '$', '`':
			return "", errors.New("unsupported character inside string: " + string(r))
		}
		buf.WriteRune(r)
	}
	return "", ErrUnterminatedString
}
