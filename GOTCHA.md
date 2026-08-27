# Gotchas

## Buck2 and Go 1.27

Problem: Buck2 fails on linux/amd64 while it links `gobuckify`.

Cause: Go 1.27 `sync/atomic` assembly calls `internal/runtime/atomic`. Buck2
adds `go tool asm -std` only when the Go toolchain reports its version. Both
the stock and custom system Go toolchains report no version.

Solution: Set `assembler_flags = ["-std"]` in
`toolchains/configurable_system_go_toolchain.bzl`. This local toolchain needs
the setting until Buck2 provides a versioned system Go toolchain with a race
option. The
[upstream toolchain](https://github.com/facebook/buck2/commit/26d2b38fac60e298944fc351a60281522214bf49)
does not provide either feature at this time.
