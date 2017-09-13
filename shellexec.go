package shellexec

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

var ErrUnknownEscSeq = errors.New("unknown escape sequence")

// Command parses line as a pseudo-shell command and returns
// the os/exec.Cmd struct to execute the line.
func Command(line string) (*exec.Cmd, error) {
	toks, err := scan(line)
	if err != nil {
		return nil, err
	}
	envIndex := 0
	for _, f := range toks {
		if !strings.ContainsRune(f, '=') {
			break
		}
		envIndex++
	}
	env := append(os.Environ(), toks[:envIndex]...)
	toks = toks[envIndex:]

	if len(toks) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := exec.Command(toks[0], toks[1:]...)
	cmd.Env = env
	return cmd, nil
}

func scan(s string) (tokens []string, err error) {
	var buf bytes.Buffer
	esc := false
	quot := false
	for _, r := range s {
		if quot {
			if r == '\'' {
				quot = false
			} else {
				buf.WriteRune(r)
			}
			continue
		}
		if esc {
			switch r {
			case '|', '&', ';', '<', '>', '(', ')', '$',
				'`', '\\', '"', '\'', ' ', '\t', '\n':
				buf.WriteRune(r)
			default:
				return nil, ErrUnknownEscSeq
			}
			esc = false
			continue
		}
		if unicode.IsSpace(r) {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
			continue
		}
		switch r {
		case '\'':
			quot = true
		case '\\':
			esc = true
			continue
		case '|', '&', ';', '<', '>', '(', ')', '$', '`', '"':
			return nil, errors.New("unsupported character: " + string(r))
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}
	return tokens, nil
}
