// Package pulumi runs the Pulumi CLI, and holds the two checks every caller
// has to make before it does.
//
// One runner rather than one per command. The two commands in this kit each
// had their own — one setting cmd.Dir, the other passing --cwd — and they
// disagreed about more than that: only one validated the stack name it handed
// to exec, and only one said anything useful when the directory held no
// Pulumi project.
package pulumi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Binary is the CLI this shells out to. A literal, and the only one that
// reaches exec.
const Binary = "pulumi"

// NonInteractive is prepended to every invocation. A CLI that stops to ask a
// question inside a task is a task that hangs with nobody watching.
const NonInteractive = "--non-interactive"

// ErrNotInstalled is returned when the CLI these tools drive is not on PATH.
//
// Its own error because a caller may want to tell it apart: a missing CLI is
// an unprepared machine, not a bad argument, and a task that reports it as
// one sends the operator looking at their own command line.
var ErrNotInstalled = errors.New("the pulumi CLI is not on PATH")

// InstallURL is where to get it, in the message rather than in a comment: the
// operator reading that error is the one who has to act on it.
const InstallURL = "https://www.pulumi.com/docs/install/"

// Require refuses before anything runs when the CLI is absent.
//
// Without it the failure arrived wrapped in whatever the caller was doing —
// `read the stack's resources: pulumi --stack dev stack --show-urns --output
// json: exec: "pulumi": executable file not found in $PATH` — which reads as
// though the command ran and could not read the stack. It had not run at all.
func Require() error {
	if _, err := exec.LookPath(Binary); err != nil {
		return fmt.Errorf("%w. These tools drive it rather than reimplementing it: install it from %s",
			ErrNotInstalled, InstallURL)
	}

	return nil
}

// StackName is what may be handed to the CLI as --stack.
//
// Validated rather than annotated. The name comes straight from an operator —
// `task plan stack=<anything>` — and reaches an exec. Pulumi's own stack names
// allow letters, digits, hyphens, underscores and periods; a leading period is
// refused, which is what keeps `..` out.
var StackName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)

// StackNameExtra names the rest for the error, so the message and the pattern
// cannot drift apart in a reader's head.
const StackNameExtra = "hyphens, underscores and periods, and not a leading period"

// ValidateStackName refuses a name the CLI should never be given.
func ValidateStackName(name string) error {
	if !StackName.MatchString(name) {
		return fmt.Errorf("%q is not a stack name: letters, digits, %s", name, StackNameExtra)
	}

	return nil
}

// ProjectFile is what makes a directory a Pulumi project.
const ProjectFile = "Pulumi.yaml"

// ProjectDirectory resolves the directory the CLI is pointed at.
//
// Absolute, because --cwd needs one. Pulumi.yaml is checked here because the
// CLI's own answer to a directory without one is a message about the current
// project rather than about the argument it was given.
func ProjectDirectory(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", dir, err)
	}

	if _, err := os.Stat(filepath.Join(absolute, ProjectFile)); err != nil {
		return "", fmt.Errorf("%s is not a Pulumi project: %w", absolute, err)
	}

	return absolute, nil
}

// Run runs the CLI in dir, returning stdout and folding stderr into the error
// so the CLI's own diagnosis reaches the operator.
func Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	// A backstop rather than the primary check: each command calls Require
	// first so the message stands on its own. This one is for a caller that
	// reaches Run directly and would otherwise get the wrapped exec error.
	if err := Require(); err != nil {
		return nil, err
	}

	// #nosec G204,G702 -- the binary is a literal, the remaining arguments are
	// literals from this package plus a stack name checked by
	// ValidateStackName and a directory checked by ProjectDirectory, and
	// CommandContext takes an argument vector: there is no shell to interpret
	// any of it. The taint analysis cannot see that the vector form is the
	// mitigation.
	command := exec.CommandContext(ctx, Binary, append([]string{NonInteractive}, args...)...)
	command.Dir = dir

	var stderr bytes.Buffer

	command.Stderr = &stderr

	out, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return nil, fmt.Errorf("pulumi %s: %w", strings.Join(args, " "), err)
		}

		return nil, fmt.Errorf("pulumi %s: %w\n%s", strings.Join(args, " "), err, message)
	}

	return out, nil
}
