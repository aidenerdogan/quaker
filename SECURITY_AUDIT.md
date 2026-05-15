# Quaker Security Audit Notes

Quaker combines macOS maintenance behavior with a Quaker-specific memory, rules, profiles, hooks, and scheduling layer.

## High-Risk Areas

- permanent deletion paths
- app uninstall leftovers
- privileged cleanup using sudo
- shell hooks
- scheduled jobs
- path/glob rules
- generated launchd plists

## Current Controls

- cleanup-style commands support `--dry-run`
- schedules run profiles with `--dry-run` by default
- protected rules are synced into the underlying cleanup whitelist before wrapped cleanup commands run
- hooks receive JSON payloads on stdin and are time-limited
- memory records capture command, args, dry-run status, result, timestamp, and source
- Quaker uses its own update guidance and does not call another product updater

## Known Follow-Ups

- broaden Quaker-specific shell tests when Bats is available
- add structured parsing for rules/profiles if the YAML format grows
- add first-class binary release packaging for `analyze-go` and `status-go`
- add GUI/TUI legal notice screens when those commercial products are created
