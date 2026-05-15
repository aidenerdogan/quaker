# Contributing to Quaker CLI

Thanks for helping improve Quaker.

Quaker CLI is open-source and MIT licensed.

## Development

Install Go:

```bash
brew install go@1.25
```

Build helper binaries:

```bash
PATH=/opt/homebrew/opt/go@1.25/bin:$PATH make build
```

Run checks:

```bash
bash -n qk quaker
PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/quaker-gocache GOMODCACHE=/private/tmp/quaker-gomodcache go test ./...
```

Prefer `--dry-run` for any command that could remove files.

## Product Direction

Quaker focuses on remembered, rule-driven maintenance:

- local memory
- policy rules
- profiles
- hooks
- suggest-only schedules
- transparent recommendations

Please keep changes aligned with that safety-first direction.
