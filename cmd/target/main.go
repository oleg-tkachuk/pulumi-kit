// Command target resolves a resource name into the URNs `pulumi --target`
// takes.
//
// Flag parsing and printing only; the matching is internal/pkg/target's.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/target"
)

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)

	// -h is a question that was answered, not a failure. flag has already
	// printed the usage by the time this returns.
	if errors.Is(err, flag.ErrHelp) {
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// options are the inputs, so that adding one does not change an argument
// position every caller has hard-coded.
type options struct {
	dir          string
	stack        string
	groupPackage string
	list         bool
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	opts, selector, err := parse(args, errOut)
	if err != nil {
		return err
	}

	// Before the arguments are judged: an absent CLI makes the whole command
	// impossible, and reporting a stack name instead sends the operator to
	// their own command line.
	if missing := pulumi.Require(); missing != nil {
		return missing
	}

	if nameErr := pulumi.ValidateStackName(opts.stack); nameErr != nil {
		return nameErr
	}

	directory, err := pulumi.ProjectDirectory(opts.dir)
	if err != nil {
		return err
	}

	resources, err := target.Resources(ctx, directory, opts.stack)
	if err != nil {
		return err
	}

	// Answering the question the refusal used to answer. Until this existed,
	// seeing what a stack holds meant mistyping a selector on purpose — the
	// list was only ever reachable through an error.
	if opts.list {
		for _, name := range target.Names(resources) {
			fmt.Fprintln(out, name)
		}

		return nil
	}

	urns, err := target.MatchAll(resources, selector, opts.groupPackage)
	if err != nil {
		return err
	}

	for _, urn := range urns {
		fmt.Fprintln(out, urn)
	}

	return nil
}

// parse reads the flags, separated so that every refusal below happens before
// anything reaches the Pulumi CLI.
func parse(args []string, errOut io.Writer) (options, string, error) {
	var opts options

	flags := flag.NewFlagSet("target", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&opts.dir, "dir", ".", "the Pulumi project directory")
	flags.StringVar(&opts.stack, "stack", "", "the stack to read (required)")
	flags.StringVar(&opts.groupPackage, "group-package", "",
		"the package a component's type token begins with, for "+target.GroupPrefix+" selectors")
	flags.BoolVar(&opts.list, "list", false,
		"print what the stack holds, as Type:name, and take no selector")
	flags.Usage = func() {
		fmt.Fprint(errOut, "usage: target [flags] <selector[,selector…]>\n"+
			"       target [flags] -list\n"+
			"  selector: a resource name, Type:name, or "+target.GroupPrefix+"Type\n"+
			"            several, comma-separated, resolve to every one of their URNs\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return options{}, "", err
	}

	// -list answers a question rather than resolving a name, so it takes no
	// selector — and refuses one rather than ignoring it, which would leave a
	// caller believing it had filtered the list.
	if opts.list {
		if flags.NArg() != 0 {
			return options{}, "", errors.New("-list takes no selector")
		}

		return opts, "", nil
	}

	if flags.NArg() != 1 {
		flags.Usage()

		return options{}, "", errors.New("exactly one selector argument")
	}

	selector := flags.Arg(0)

	if strings.TrimSpace(selector) == "" {
		return options{}, "", target.ErrNoSelector
	}

	return opts, selector, nil
}
