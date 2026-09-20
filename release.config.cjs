// Computes the next SemVer from Conventional Commits and creates the git tag —
// nothing else. Run by .github/workflows/release.yml, which ci.yaml dispatches
// once every check on a push to main has passed.
//
// A Go module needs no publish step: the module proxy serves whatever the tag
// points at, so the tag IS the release. The GitHub release beside it is notes
// for people, built from merged pull requests by .github/release.yml.
//
// Plain `conventionalcommits` preset, no custom releaseRules: feat is a minor,
// fix/perf/revert are a patch, a `!` or `BREAKING CHANGE:` footer is a major,
// and docs/style/refactor/test/build/ci/chore release nothing on their own.
module.exports = {
  branches: ["main"],
  tagFormat: "v${version}",
  repositoryUrl: "https://github.com/oleg-tkachuk/pulumi-kit.git",
  plugins: [["@semantic-release/commit-analyzer", { preset: "conventionalcommits" }]],
};
