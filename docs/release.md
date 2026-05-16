# Release

GoReleaser builds `gomeshcomd` from `cmd/gomeshcomd/main.go`.

Release binaries receive version metadata through linker flags:

- `version`
- `commit`
- `date`

Local builds still use `go build ./cmd/gomeshcomd`.
