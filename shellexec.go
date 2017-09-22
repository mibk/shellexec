package shellexec

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
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
	s := parser{s: line, getenv: os.Getenv}

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
	buf    bytes.Buffer
	s      string
	getenv func(key string) string
}

func (p *parser) parseLine() (cmd, error) {
	var c cmd
	for len(p.s) > 0 {
		r, size := utf8.DecodeRuneInString(p.s)
		if unicode.IsSpace(r) {
			p.s = p.s[size:]
			continue
		}

		if c.cmd == "" {
			v, err := p.parseVarAssign()
			if err == errNotAssign {
				c.cmd = v
			} else if err != nil {
				return cmd{}, err
			} else {
				c.env = append(c.env, v)
			}
		} else {
			f, err := p.parseField()
			if err != nil {
				return cmd{}, err
			}
			c.args = append(c.args, f)
		}
	}
	if c.cmd == "" {
		return cmd{}, ErrEmptyCommand
	}
	return c, nil
}

var errNotAssign = errors.New("not assignment")

func (p *parser) parseVarAssign() (string, error) {
	v := p.parseIdent()
	err := errNotAssign
	if v != "" && p.s != "" && p.s[0] == '=' {
		err = nil
	}

	f, err2 := p.parseField()
	if err2 != nil {
		return "", err2
	}
	return v + f, err
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
		case '$':
			v, err := p.parseVarExpr()
			if err != nil {
				return "", err
			}
			p.buf.WriteString(v)
			continue
		case '|', '&', ';', '<', '>', '(', ')', '`',
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
	for len(p.s) > 0 {
		r, size := utf8.DecodeRuneInString(p.s)
		p.s = p.s[size:]

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
			return buf.String(), nil
		case '\\':
			esc = true
			continue
		case '$':
			v, err := p.parseVarExpr()
			if err != nil {
				return "", err
			}
			buf.WriteString(v)
			continue
		case '`':
			return "", errors.New("unsupported character inside string: " + string(r))
		}
		buf.WriteRune(r)
	}
	return "", ErrUnterminatedString
}

func (p *parser) parseVarExpr() (string, error) {
	if p.s != "" {
		switch p.s[0] {
		case '(':
			return "", errors.New("command substitution or arithmetic expansion not supported")
		case '{':
			return "", errors.New("parameter expansion not supported")
		case '@', '*', '#', '?', '-', '$', '!', '0':
			return "", errors.New("special parameters not supported")
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return "", errors.New("positional parameters not supported")
		}
	}

	name := p.parseIdent()
	if name == "" {
		return "$", nil
	}
	return p.getenv(name), nil
}

func (p *parser) parseIdent() string {
	var i int
	for i < len(p.s) {
		r, size := utf8.DecodeRuneInString(p.s[i:])
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' ||
			r == '_' || i > 0 && r >= '0' && r <= '9') {
			break
		}
		i += size
	}
	v := p.s[:i]
	p.s = p.s[i:]
	return v
}
