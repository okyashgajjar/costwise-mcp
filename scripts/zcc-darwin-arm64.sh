#!/bin/sh
# Zig CC wrapper for macOS arm64 cross-compilation.
# GoReleaser requires CC to be a single executable — zig cc with
# arguments cannot be passed as an env var without splitting issues.
exec zig cc -target aarch64-macos "$@"
