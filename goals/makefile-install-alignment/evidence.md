# Evidence: Makefile install alignment

Verdict: PASS

Date: 2026-10-20. Re-verified after the goal work changed the build path and binary version.

## Alignment

`make build` runs the CGO-disabled, trimmed, version-stamped build. `make install` delegates to the hardened build script with `--release --install=$(BINDIR)`; it does not run a second, divergent install path.

```text
$ make -n build
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/jadmadi/swapaudit/internal/version.Version=... -X ...Commit=... -X ...Date=..." -o swapaudit github.com/jadmadi/swapaudit/cmd/swapaudit

$ make -n install
./scripts/build --release --install="/home/jad/.local/bin"
```

## Verification

```text
$ ./scripts/check.sh --full
Building swapaudit (release mode) for linux/amd64...
✔ Built binary self-verified: swapaudit 2e266e2-dirty (commit: 2e266e2, ...)
✔ All checks passed.
```

The release build path compiles with `CGO_ENABLED=0`, produces a working binary, and self-verifies its version metadata. No Makefile target bypasses `scripts/build`.
