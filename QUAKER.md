# Quaker Product Notes

Quaker CLI is the open-source engine for a broader Quaker product line.

The intended structure is open-core:

- **Quaker CLI**: open-source MIT project.
- **Quaker GUI/TUI**: commercial applications may be developed separately and distributed under their own terms.

Quaker is a Quaker-owned macOS maintenance engine. Its distinguishing layer is local memory, policy rules, profiles, hooks, suggest-only schedules, recommendation flows, Quaker-branded CLI entrypoints, and an architecture intended for future GUI/TUI products.

This project should be described as:

> Remembered, rule-driven Mac maintenance with local policy, profiles, hooks, and suggest-only automation.

Keep the CLI focused on Quaker-owned behavior: local memory, policy, profiles, hooks, schedules, dry-run-first cleanup, and clear reporting.
