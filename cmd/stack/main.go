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

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/stack"
)

// Exit codes. `exists` answers with its status, so "the stack is not there" has
// to be distinguishable from "the question could not be answered" — a caller
// that cannot tell them apart creates a stack because the network was down.
const (
	ExitFailed = 1
	ExitAbsent = 2
)

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout)
	if err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(exitCode(err))
}

// exitCode maps a failure onto the status a shell reads, separated from main so
// the mapping can be tested: `if stack exists …` treating a backend failure as
// absence is the bug this exists to prevent, and it is a one-line mistake.
func exitCode(err error) int {
	if errors.Is(err, stack.ErrNotFound) {
		return ExitAbsent
	}

	return ExitFailed
}

// The three commands, named once because the switch below and the two messages
// that list them must not drift apart.
const (
	CommandExists = "exists"
	CommandEnsure = "ensure"
	CommandRef    = "ref"
)

// Commands is the set, in the order the messages list them.
var Commands = []string{CommandExists, CommandEnsure, CommandRef}

// Usage is what a caller sees, whether it was asked for or not.
var Usage = fmt.Sprintf("usage: stack <%s> <project-dir> <stack>", strings.Join(Commands, "|"))

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

	// Before anything else, and after help: an absent CLI makes every command
	// below impossible, whatever the arguments say.
	if err := pulumi.Require(); err != nil {
		return err
	}

	// A wrong argument list stays an error: that is a caller with a bug, not a
	// person asking a question.
	if len(args) != Arity {
		return errors.New(Usage)
	}

	command, dir, name := args[0], args[1], args[2]

	if err := pulumi.ValidateStackName(name); err != nil {
		return err
	}

	switch command {
	case CommandExists:
		return exists(ctx, dir, name)

	case CommandEnsure:
		state, err := stack.Ensure(ctx, dir, name)
		if err != nil {
			return err
		}

		fmt.Fprintln(out, state)

		return nil

	case CommandRef:
		reference, err := stack.Reference(ctx, dir, name)
		if err != nil {
			return err
		}

		fmt.Fprintln(out, reference)

		return nil

	default:
		return fmt.Errorf("unknown command %q: %s", command, strings.Join(Commands, ", "))
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
		// Deliberately not wrapping ErrNotFound: Exists said the stack was
		// absent and then the listing failed, so the second failure is what
		// the caller has to see. Reporting absence here would be a guess.
		return fmt.Errorf("%s has no stack named %q, and listing the others failed: %w",
			dir, name, listErr)
	}

	return fmt.Errorf("%w: nothing named %q in %s. Stacks that do exist: %s",
		stack.ErrNotFound, name, dir, strings.Join(available, ", "))
}
