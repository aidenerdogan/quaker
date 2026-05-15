# Quaker

Remembered, rule-driven Mac maintenance for people who want cleanup to be transparent, repeatable, and calm.

Quaker CLI is an open-source macOS maintenance engine. It combines safe cleanup, uninstall, optimization, disk analysis, system status, project purge, installer cleanup, Touch ID setup, shell completions, and a Quaker-specific memory and policy layer.

Quaker is its own macOS maintenance tool with a Quaker-owned command surface, state model, policy layer, and automation workflow.

## What Quaker Adds

- **Local memory**: every scan, dry run, delete, warning, failure, and user decision can be recorded under `~/.quaker/memory.jsonl`.
- **Rules**: protect important paths, ignore noisy suggestions, and keep policy separate from one-off commands.
- **Profiles**: save repeatable cleanup recipes from useful runs.
- **Hooks**: run user scripts after key events such as scans, cleanup, uninstall, and doctor checks.
- **Schedules**: create suggest-only `launchd` jobs that scan and recommend by default.
- **Doctor**: summarize disk pressure, rules, memory, permissions, and next useful actions.
- **Open-core shape**: the CLI remains open-source; future GUI/TUI products can build on the same engine.

## Install

From this checkout:

```bash
./install-quaker.sh
```

By default this installs:

- `qk` and `quaker` into `~/.local/bin`
- application files into `~/.local/share/quaker`
- local state into `~/.quaker`

Build the Quaker CLI:

```bash
PATH=/opt/homebrew/opt/go@1.25/bin:$PATH make build
```

## Usage

```bash
qk --help
qk doctor
qk clean --dry-run
qk optimize --dry-run
qk uninstall --dry-run "App Name"
qk analyze --json ~/Downloads
qk status --json
qk purge --dry-run
qk installer --dry-run
```

Memory:

```bash
qk memory list
qk memory show <id>
qk memory export --format json
qk memory forget <id>
```

Rules:

```bash
qk rules add protect ~/Projects/important
qk rules add ignore old-cache-suggestion
qk rules list
qk rules check ~/Projects/important/cache/file.tmp
qk rules remove <rule-id>
```

Profiles:

```bash
qk profile create weekly-safe --from-last-run
qk profile list
qk profile run weekly-safe --dry-run
```

Automation:

```bash
qk schedule add weekly-scan --profile weekly-safe
qk schedule list
qk schedule remove weekly-scan-weekly-safe
```

Hooks:

```bash
qk hooks install after-doctor ./scripts/my-hook.sh
qk hooks list
```

Completions:

```bash
qk completion zsh
qk completion bash
qk completion fish
```

## Safety Defaults

- Prefer `--dry-run` first.
- Scheduled jobs are suggest-only by default.
- Protected rules override profiles and schedules.
- Cleanup-style commands are routed through Quaker memory where possible.
- Hooks receive JSON on stdin and are time-limited.

## Development

Install Go with Homebrew:

```bash
brew install go@1.25
```

Run checks:

```bash
bash -n qk quaker
PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/quaker-gocache GOMODCACHE=/private/tmp/quaker-gomodcache go test ./...
```

## License

Quaker CLI is distributed under the MIT License.
