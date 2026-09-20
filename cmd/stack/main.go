// Command stack answers and settles questions about Pulumi stacks.
//
// Argument parsing and printing only; the decisions are
// internal/pkg/stack's.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/stack"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Usage is what a caller sees, whether it was asked for or not.
const Usage = "usage: stack <exists|ensure|ref> <project-dir> <stack>"

// HelpFlags are the ways a caller asks what this takes.
//
// Answering with an error was the defect: cmd/target exits zero for -h and
// this did not, so two commands in one kit disagreed — and the message arrived
// on stderr under an `error:` prefix, which says the request failed when it
// was answered.
var HelpFlags = map[string]bool{"-h": true, "--help": true, "help": true}

// Arity is how many arguments the three commands take.
const Arity = 3

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 1 && HelpFlags[args[0]] {
		fmt.Fprintln(out, Usage)

		return nil
	}

	// A wrong argument list stays an error: that is a caller with a bug, not a
	// person asking a question.
	if len(args) != Arity {
		return errors.New(Usage)
	}

	command, dir, name := args[0], args[1], args[2]

	switch command {
	case "exists":
		return exists(ctx, dir, name)

	case "ensure":
		state, err := stack.Ensure(ctx, dir, name)
		if err != nil {
			return err
		}

		fmt.Fprintln(out, state)

		return nil

	case "ref":
		reference, err := stack.Reference(ctx, dir, name)
		if err != nil {
			return err
		}

		fmt.Fprintln(out, reference)

		return nil

	default:
		return fmt.Errorf("unknown command %q: exists, ensure or ref", command)
	}
}

// exists is the one command whose answer is its exit code, so the absent case
// has to build the message itself.
func exists(ctx context.Context, dir, name string) error {
	present, err := stack.Exists(ctx, dir, name)
	if err != nil {
		return err
	}

	if present {
		return nil
	}

	// Listing what does exist, because the failure this replaces gave only an
	// exit code.
	available, listErr := stack.Names(ctx, dir)
	if listErr != nil {
		return fmt.Errorf("no stack named %q in %s (and listing failed: %w)", name, dir, listErr)
	}

	return fmt.Errorf("no stack named %q in %s. Stacks that do exist: %s",
		name, dir, strings.Join(available, ", "))
}
