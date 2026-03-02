package safety

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func IsForceEnabled(opts RunOptions) bool {
	if opts.Force {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SEED_FORCE")))
	return v == "1" || v == "true" || v == "yes"
}

func EnforceSafety(opts RunOptions, res PreflightResult, in io.Reader, out io.Writer) error {
	forceEnabled := IsForceEnabled(opts)

	if res.IsDestructive && !forceEnabled {
		return errors.New("destructive operation refused: pass --force or set SEED_FORCE=1")
	}

	if !res.HasHighRisk() {
		return nil
	}

	if opts.Yes {
		return nil
	}

	if !isTTY() {
		return errors.New("high-risk operation requires explicit confirmation with --yes in non-interactive mode")
	}

	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	_, _ = fmt.Fprintln(out, "Risk detected. Type 'yes' to continue:")
	s := bufio.NewScanner(in)
	if !s.Scan() {
		return errors.New("confirmation aborted")
	}
	if strings.ToLower(strings.TrimSpace(s.Text())) != "yes" {
		return errors.New("confirmation declined")
	}
	return nil
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
