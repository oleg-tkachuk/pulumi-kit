package target

import (
	"context"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
)

// ErrNotInstalled is returned by Resolve and List when the pulumi CLI is not
// on PATH: a machine that is not prepared, not a bad argument.
var ErrNotInstalled = pulumi.ErrNotInstalled

// Stack names the stack to read: the project directory holding Pulumi.yaml,
// and the stack's name.
type Stack struct {
	Dir  string
	Name string
}

// Resolve returns the URNs selectors name in the stack, for one `--target`
// each — the library form of `target -dir … -stack … <selectors>`.
//
// It makes the checks the command makes, in the same order, before the CLI
// runs: the CLI is installed, the name is a stack name, the directory is a
// project. Then every selector has to resolve, as MatchAll describes;
// groupPackage is required only when a selector is a group.
func Resolve(ctx context.Context, stack Stack, selectors, groupPackage string) ([]string, error) {
	resources, err := read(ctx, stack)
	if err != nil {
		return nil, err
	}

	return MatchAll(resources, selectors, groupPackage)
}

// List returns what the stack holds as selectors, one each, as Names does —
// the library form of `target -list`.
func List(ctx context.Context, stack Stack) ([]string, error) {
	resources, err := read(ctx, stack)
	if err != nil {
		return nil, err
	}

	return Names(resources), nil
}

// read makes the command's checks and reads the stack. One function, so the
// command and the library refuse the same inputs with the same words.
func read(ctx context.Context, stack Stack) ([]Resource, error) {
	if err := pulumi.Require(); err != nil {
		return nil, err
	}

	if err := pulumi.ValidateStackName(stack.Name); err != nil {
		return nil, err
	}

	directory, err := pulumi.ProjectDirectory(stack.Dir)
	if err != nil {
		return nil, err
	}

	return Resources(ctx, directory, stack.Name)
}
