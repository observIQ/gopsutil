# Contributing

This is Bindplane's fork of [`shirou/gopsutil`](https://github.com/shirou/gopsutil). Contributions are welcome.

## API compatibility

This fork attempts to stay API compatible with upstream `shirou/gopsutil`. The way we do that is by keeping changes additive. Prefer new methods, new fields, new struct types, and new map keys over changing or removing what already exists. Additive changes are a superset, so existing consumers keep working and simply ignore what they do not use.

Compatibility is a goal, not a guarantee. If a change cannot be made additively, or upstream behavior is buggy enough that matching it would be wrong, a breaking change may be warranted. Call it out explicitly in the issue and the PR description so it can be reviewed as a deliberate divergence.

## Development flow

Work happens on feature branches off `master`. A change can be a single PR or a stack of PRs. We use and recommend [git-town](https://www.git-town.com/) for building and managing stacks. It is purely client-side, so it needs no special account or hosted service. git-town is not required. You can build a stack with plain git if you prefer.

Before you propose a PR, make sure it builds and tests cleanly:

- `go test ./...` for the packages you touched.
- `make build_test` to confirm the change cross-compiles for every supported `GOOS`/`GOARCH`.
- `make vet` and golangci-lint for static checks. Run `gofmt` on changed files.

Some code is platform-gated with build tags (for example `//go:build aix`). Cross-compiling proves it builds, but value tests for that platform must be run on that platform. CI cannot run AIX-gated tests on a Linux runner, for example.

## How we take contributions

We accept contributions as a single PR or as a stack of PRs. The process:

1. **Open an issue** describing the work. The issue is the anchor for the change.
2. **Build your change** against that issue. A small change can be one PR. Larger work can be a stack. We recommend git-town for stacks (`git town hack`, `git town append`, `git town propose`), but plain git works too.
3. **Reference the issue.** A single PR, or the final PR in a stack, includes `resolves <ISSUE_TAG>` so that merging it closes the issue. Any earlier PR in a stack includes `partially addresses <ISSUE_TAG>` instead, where `<ISSUE_TAG>` is the issue reference (for example `#123`).

We suggest `resolves` because it reads well for both bug fixes and feature requests. GitHub's other closing keywords (`closes`, `fixes`) work too. The important part is that `partially addresses` is not a closing keyword, so earlier PRs in a stack reference the issue without closing it. Only the closing keyword on the final PR closes the issue.

## Generative AI

If generative AI tooling assisted significantly in authoring a contribution, disclose it with an `Assisted-by:` commit message trailer that names the model used.

For example:

```
Assisted-by: Claude Opus 4.8
```
