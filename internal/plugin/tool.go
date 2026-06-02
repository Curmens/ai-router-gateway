package plugin

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
)

type ExternalTool interface {
	Execute(ctx context.Context, args []string) (string, error)
}

var safeArgRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.\/\\:]+$`)

type CLIExecWrapper struct {
	BinaryPath string
}

func NewCLIExecWrapper(binaryPath string) *CLIExecWrapper {
	return &CLIExecWrapper{BinaryPath: binaryPath}
}

func (w *CLIExecWrapper) Execute(ctx context.Context, args []string) (string, error) {
	if w.BinaryPath == "" {
		return "", errors.New("empty binary path")
	}

	for _, arg := range args {
		if !safeArgRegex.MatchString(arg) {
			return "", fmt.Errorf("unsafe argument detected: %q. Arguments must be alphanumeric or contain safe characters only", arg)
		}
	}

	cmd := exec.CommandContext(ctx, w.BinaryPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %w. stderr/stdout: %s", err, string(output))
	}

	return string(output), nil
}
