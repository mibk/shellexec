package shellexec

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"unicode"
	"unicode/utf8"
)

// Command parses line using a shell-like syntax and returns
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
	last   rune
	peeked *rune
	getenv func(key string) string

	identBuf bytes.Buffer
}

const eof rune = utf8.MaxRune + 1

func (p *parser) next() rune {
	if p.peeked != nil {
		r := *p.peeked
		p.peeked = nil
		return r
	}

	if len(p.s) == 0 {
		p.last = eof
		return eof
	}
	var size int
	p.last, size = utf8.DecodeRuneInString(p.s)
	p.s = p.s[size:]

	if p.last == '\\' && p.s != "" && p.s[0] == '\n' {
		// line continuation; remove it from the input
		p.s = p.s[1:]
		return p.next()
	}

	return p.last
}

func (p *parser) backup() {
	p.peeked = &p.last
}

func (p *parser) token() string {
	t := p.buf.String()
	p.buf.Reset()
	return t
}

func (p *parser) parseLine() (cmd, error) {
	var c cmd
loop:
	for {
		r := p.next()
		switch {
		case unicode.IsSpace(r):
			continue
		case r == eof:
			break loop
		}
		p.backup()

		if c.cmd == "" {
			if err := p.parseVarAssign(); err == errNotAssign {
				c.cmd = p.token()
			} else if err != nil {
				return cmd{}, err
			} else {
				c.env = append(c.env, p.token())
			}
		} else {
			if err := p.parseField(); err != nil {
				return cmd{}, err
			}
			c.args = append(c.args, p.token())
		}
	}
	if c.cmd == "" {
		return cmd{}, errors.New("empty command")
	}
	return c, nil
}

var errNotAssign = errors.New("not assignment")

func (p *parser) parseVarAssign() error {
	v := p.parseIdent()
	p.buf.WriteString(v)
	err := errNotAssign
	if v != "" && p.next() == '=' {
		err = nil
	}
	p.backup()

	if err := p.parseField(); err != nil {
		return err
	}
	return err
}

func (p *parser) parseField() error {
	esc := false
	for {
		r := p.next()
		if r == eof {
			break
		}

		if esc {
			p.buf.WriteRune(r)
			esc = false
			continue
		}
		if unicode.IsSpace(r) {
			break
		}
		switch r {
		case '\'':
			if err := p.parseSingleQuotes(); err != nil {
				return err
			}
		case '"':
			if err := p.parseDoubleQuotes(); err != nil {
				return err
			}
		case '\\':
			esc = true
			continue
		case '$':
			if err := p.parseVarExpr(); err != nil {
				return err
			}
			p.backup()
			continue
		case '|', '&', ';', '<', '>', '(', ')', '`',
			// Forbid these characters as they may need to be
			// quoted under certain circumstances.
			'*', '?', '[', '#', '~':
			return errors.New("unsupported character: " + string(r))
		default:
			p.buf.WriteRune(r)
		}
	}
	return nil
}

func (p *parser) parseSingleQuotes() error {
	for {
		switch r := p.next(); r {
		case '\'':
			return nil
		case eof:
			return errors.New("string not terminated")
		default:
			p.buf.WriteRune(r)
		}
	}
	panic("unreachable")
}

func (p *parser) parseDoubleQuotes() error {
	var esc bool
	for {
		r := p.next()
		if r == eof {
			return errors.New("string not terminated")
		}

		if esc {
			switch r {
			default:
				p.buf.WriteRune('\\')
				fallthrough
			case '$', '`', '"', '\\':
				p.buf.WriteRune(r)
			}
			esc = false
			continue
		}
		switch r {
		case '"':
			return nil
		case '\\':
			esc = true
			continue
		case '$':
			if err := p.parseVarExpr(); err != nil {
				return err
			}
			continue
		case '`':
			return errors.New("unsupported character inside string: " + string(r))
		}
		p.buf.WriteRune(r)
	}
	panic("unreachable")
}

func (p *parser) parseVarExpr() error {
	switch p.next() {
	case '(':
		return errors.New("command substitution or arithmetic expansion not supported")
	case '{':
		return errors.New("parameter expansion not supported")
	case '@', '*', '#', '?', '-', '$', '!', '0':
		return errors.New("special parameters not supported")
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return errors.New("positional parameters not supported")
	}
	p.backup()

	v := "$"
	if name := p.parseIdent(); name != "" {
		v = p.getenv(name)
	}
	p.buf.WriteString(v)
	return nil
}

func (p *parser) parseIdent() string {
	p.identBuf.Reset()
	for {
		r := p.next()
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_' ||
			p.identBuf.Len() > 0 && r >= '0' && r <= '9') {
			p.backup()
			return p.identBuf.String()
		}
		p.identBuf.WriteRune(r)
	}
	panic("unreachable")
}
