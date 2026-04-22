package croc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"qim-data/internal/config"
)

var versionRegex = regexp.MustCompile(`v(\d+)`)

// ResolveBinary returns the croc executable path.
func ResolveBinary(cfg config.Config, override string) (string, error) {
	if override != "" {
		if err := isExecutable(override); err != nil {
			return "", err
		}
		return override, nil
	}

	if cfg.CrocPath != "" {
		if err := isExecutable(cfg.CrocPath); err == nil {
			return cfg.CrocPath, nil
		}
	}

	path, err := exec.LookPath("croc")
	if err != nil {
		return "", errors.New("croc not found in PATH; install croc v10+ or set --croc-path")
	}
	return path, nil
}

func isExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected croc binary", path)
	}
	return nil
}

// Version returns the raw `croc --version` output.
func Version(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s --version: %w (%s)", path, err, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}

// ParseMajor extracts major version from a version string like "croc version v10.4.2".
func ParseMajor(versionOutput string) (int, bool) {
	match := versionRegex.FindStringSubmatch(versionOutput)
	if len(match) != 2 {
		return 0, false
	}
	var major int
	_, err := fmt.Sscanf(match[1], "%d", &major)
	if err != nil {
		return 0, false
	}
	return major, true
}

// CheckRelayDial tries TCP connection to relay endpoint.
func CheckRelayDial(relay string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", relay, timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// Run executes croc with inherited stdio.
func Run(path string, args []string, extraEnv map[string]string, redactions []string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin

	env := os.Environ()
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	if len(redactions) == 0 {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 2)
	go func() { done <- copyWithRedaction(os.Stdout, stdoutPipe, redactions) }()
	go func() { done <- copyWithRedaction(os.Stderr, stderrPipe, redactions) }()

	var copyErr error
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil && copyErr == nil {
			copyErr = err
		}
	}

	waitErr := cmd.Wait()
	if copyErr != nil {
		return copyErr
	}
	return waitErr
}

func copyWithRedaction(dst io.Writer, src io.Reader, redactions []string) error {
	replacerArgs := []string{}
	for _, value := range redactions {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		replacerArgs = append(replacerArgs, v, "[REDACTED]")
	}
	replacer := strings.NewReplacer(replacerArgs...)

	r := bufio.NewReader(src)
	for {
		chunk, err := r.ReadString('\n')
		if chunk != "" {
			chunk = replacer.Replace(chunk)
			if _, wErr := io.WriteString(dst, chunk); wErr != nil {
				return wErr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
