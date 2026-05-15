# Quaker Security Policy

Quaker is a local macOS maintenance tool. Its main risk is unintended local damage from cleanup, uninstall, optimization, purge, installer cleanup, or automation.

## Supported Branch

Security fixes target the `main` branch.

## Reporting

Please open a private report or contact the maintainer directly before publishing details about destructive-operation bypasses, unsafe path handling, privilege escalation, or release integrity issues.

Include:

- Quaker version or commit
- command run
- macOS version
- whether `--dry-run` was used
- relevant output or logs

## Safety Principles

- Prefer dry-run previews.
- Protect high-value paths by default.
- Let user-defined rules override profiles and schedules.
- Keep scheduled jobs suggest-only unless explicitly changed in a future design.
- Refuse or skip uncertain destructive operations.
- Keep release packaging and licensing accurate.
