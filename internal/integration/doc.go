// Package integration holds the kit to a real pulumi CLI rather than to output
// written by hand.
//
// Every other test in this module parses JSON somebody typed, which is what
// lets them run without a backend — and is also why none of them can notice
// the CLI changing shape. The comments that say "this only fires if that
// changes" need something that would see it change. This is that, behind the
// `integration` build tag so `go test ./...` still needs nothing installed:
//
//	go test -tags integration ./internal/integration/
//
// It needs `pulumi` on PATH and no network: the backend is a temporary
// directory, and the stack's state is imported from testdata rather than
// produced by running a program, so no language SDK or provider plugin is
// fetched.
package integration
