package shellexec

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Command parses line as a pseudo-shell command and returns
// the os/exec.Cmd struct to execute the line.
func Command(line string) (*exec.Cmd, error) {
	fs := strings.Fields(line)
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
