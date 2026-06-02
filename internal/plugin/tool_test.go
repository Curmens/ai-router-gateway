package plugin

import (
	"context"
	"strings"
	"testing"
)

func TestCLIArgumentSanitizer(t *testing.T) {
	wrapper := NewCLIExecWrapper("/bin/echo")

	safeArgs := []string{"safe-arg_1.txt", "path/to/file", "C:\\Windows\\System32"}
	for _, arg := range safeArgs {
		_, err := wrapper.Execute(context.Background(), []string{arg})
		if err != nil && strings.Contains(err.Error(), "unsafe argument") {
			t.Errorf("Expected argument %q to be verified as safe, but got error: %v", arg, err)
		}
	}

	unsafeArgs := []string{
		"un;safe",
		"args&injection",
		"bad|pipe",
		"rm -rf /",
		"`id`",
		"$(whoami)",
	}

	for _, arg := range unsafeArgs {
		_, err := wrapper.Execute(context.Background(), []string{arg})
		if err == nil || !strings.Contains(err.Error(), "unsafe argument") {
			t.Errorf("Expected argument %q to trigger unsafe detection, but it succeeded or got different error: %v", arg, err)
		}
	}
}
