# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-19

### Added
- Modular layered Go architecture with `cmd/swapaudit` and clean `internal/` packages.
- Accurate kernel memory auditing parsing `/proc/meminfo`, `/proc/[pid]/status`, and `/proc/[pid]/sched`.
- PID reuse protection via kernel start-ticks verification before SIGTERM dispatch.
- Tmpfs cache inspection in `/tmp` with symlink guards, UID verification, and mtime depth analysis.
- Responsive terminal table engine supporting automatic column dropping, ASCII mode, and NO_COLOR.
- Build tooling including GoReleaser v2 configuration, multi-mode build script, and Makefile.
- Continuous integration workflows for automated race detection, linting, and cross-compilation.
- Dual-licensing under Waqf Digital Public License (WaqfDPL-Mulzim 1.0) and GNU AGPLv3.
