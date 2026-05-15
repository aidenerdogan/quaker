## Summary

## Testing

```bash
bash -n qk quaker lib/quaker/core.sh bin/installer.sh bin/analyze.sh bin/status.sh tests/quaker_cli.bats
PATH=/opt/homebrew/opt/go@1.25/bin:$PATH GOCACHE=/private/tmp/quaker-gocache GOMODCACHE=/private/tmp/quaker-gomodcache go test ./...
```

## Notes
