# Business Review — swapaudit — 2026-10-19

## Review metadata

| Field | Value |
| --- | --- |
| Product | swapaudit |
| Repository | Local `/home/jad/scripts/active/swapaudit`, an untracked directory inside the parent `jadmadi/scripts` worktree. The public `github.com/jadmadi/swapaudit` URL returns 404. |
| Commit / branch | Parent worktree `b89ed95` / `main`; this project directory is untracked. The local binary reports `1a8479e`. |
| Review date | 2026-10-19 |
| Mode | Default (full review) |
| Focus | Full review: offer, distribution, trust, core flow, sustainability |
| Reviewer | Business review agent |
| Evidence cut-off | 2026-10-19 |

---

## 1. Verdict

**Recommendation:** **Fix the foundation** before adding any feature.

**Confidence:** **High** for the distribution and safety findings (directly observed). **Medium** for market and differentiation (inferred from code, docs, and free competitors).

**The answer in one paragraph:** swapaudit is a real, fast, safety-conscious Linux CLI that combines per-process swap attribution, tmpfs cache discovery, and agent-aware cleanup. The code works locally: race tests pass, the binary audit completed in 81 ms on this host, and the file-deletion guards are credible. But there is no public repository, no tag, no release, no package, and no funding path. Every install and upgrade command in the README currently fails, and the local project has no standalone git history. The binding constraint is distribution and trust, not features. Publish a standalone repo and a v0.1.0 release, fix the destructive process heuristics, then test the agent-aware wedge with real users.

**Why this is not a "Proceed":** a user cannot reach the product through any documented public path. A tool that cannot be installed has no activation, retention, or revenue to review.

**Why this is not a "Stop":** the core mechanism is implemented and testable, the performance claim held on this host, and the safety design is better than a shell script. The gap is fixable with release and safety work, not a rewrite.

**Top five gaps**

| # | Gap | Severity | Evidence | Why it matters |
| --- | --- | --- | --- | --- |
| 1 | No public repository, tag, or release; all documented install and upgrade paths return 404 | Critical | `README.md:3-4`, `README.md:124`, `README.md:139`, `README.md:148`; `git remote -v`; `git tag`; `gh repo list jadmadi`; `curl` 404 for repo and module proxy | No external user can install the product; adoption, feedback, and revenue are all zero |
| 2 | Project directory is untracked inside the parent repo; goal audit commit SHAs do not exist | Critical | `git status --short .` → `?? ./`; `git ls-tree -r HEAD` has no swapaudit path; `git cat-file -t` fails for `fc6640d…`, `3854d3b…`, `9170277…`; `goals/README.md:42-51` | No history, backup, code review, or reproducible CI; "verified" goal claims are not grounded in this tree |
| 3 | Orphan heuristic can classify a legitimate systemd-managed agent as orphaned and kill it | Critical | `internal/proc/proc.go:203-212`; `internal/triage/triage.go:31-36`; `internal/cleaner/cleaner.go:246` | A false-positive kill destroys a running agent session and the user's trust |
| 4 | Process termination has no UID ownership check; root can signal other users' processes | Critical | `internal/cleaner/cleaner.go:148-161`; contrast file UID check at `internal/cache/cache.go:83` | Running as root with `--clean-orphaned` or `--clean-dormant` can act across users |
| 5 | No economic buyer, revenue, or sustainability mechanism; direct substitutes are free | High | No pricing/sponsor/support references in product docs; license is WaqfDPL/AGPL; competitor sources in §8 | No funding for releases, support, or maintenance; the product must win on adoption, not price |

**What would change this verdict**

- **Move to "Narrow" or "Proceed":** a public repository with a signed `v0.1.0` release, working assets for all four architectures, at least 10 external installs, and 3–5 user reports showing the agent-aware cleanup solves a real recurring pain. That would justify focusing the roadmap on the support-engineering/agent wedge.
- **Move to "Stop" or "personal utility":** evidence that the target users do not feel this pain, or that the project is intentionally private and waqf-funded with no adoption goal. In that case the missing revenue model is not a defect, but the public claims and install instructions are still wrong and should be removed.
- **Strongest objection to this verdict:** the code works and the safety tests pass; perhaps distribution is just a packaging chore, not a business problem. The rebuttal is that the cheapest possible demand test is a public release. Until that exists, there is no evidence of demand, and no user can reach the value. A one-week release fix is the highest-leverage action available.
- **What would lower confidence:** discovering a private, actively used repository or an external distribution channel not visible in this environment. The 404s and `gh repo list` result would then be access artifacts, not product facts. The safety findings would remain.

---

## 2. Scope and evidence

**Reviewed**

- Product docs: `README.md`, `AGENTS.md`, `GOVERNANCE.md`, `SECURITY.md`, `CONTRIBUTING.md`, `MAINTAINERS.md`, `CHANGELOG.md`, `goals/README.md`.
- Code: all 21 Go files under `cmd/` and `internal/`; tests; build scripts; Makefile; `go.mod`.
- Release and CI: `.goreleaser.yaml`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `scripts/build`, `scripts/check.sh`.
- Execution: `go test -race ./...`, `go vet ./...`, `go test -cover ./...`, local `./swapaudit --version`, timed audit, `--help`.
- Repository state: git remote, tags, tracked files, commit existence, GitHub repo lookup, module proxy lookup.
- External sources: competitor docs and pricing pages listed in Appendix A, accessed 2026-10-19.

**Not reviewed**

- Landing page, pricing page, `SPEC.md`, and `prototypes/`: **none exist in this project.** The claim inventory therefore comes from `README.md`, `GOVERNANCE.md`, `SECURITY.md`, and CLI copy, not from a marketing site or prototype.
- Private repositories or private release channels. The public URL is 404; a private existence cannot be ruled out from this environment.
- Actual GitHub Actions runs, release logs, download counts, or user telemetry. There are no public releases and no analytics.
- User interviews, support tickets, or sales conversations. None exist in the repo.
- Kernel variants other than this host, distro packaging, enterprise procurement, and legal review of WaqfDPL/AGPL.
- Secrets, credentials, and `.env` files. None were read.

**Assumptions**

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| The public URL `github.com/jadmadi/swapaudit` is the intended distribution point | It is the only URL in the README, Go module path, updater, and badges | Ask the maintainer; inspect the private repo if one exists |
| The project is intended to be used by external Linux developers, not only by its author | README, license, governance, and release pipeline are written for public distribution | 5 user interviews; public release download count |
| The 81 ms audit is representative enough to treat the performance claim as verified | The claim is "sub-second"; one run is enough for that narrow claim, not for a benchmark | Run on 3 distros and with 10k `/tmp` entries |
| Free competitors are the relevant price anchor | All direct substitutes found are free or OS-bundled | Ask target users what they currently use and whether they pay |

**Evidence ledger**

| Question or claim | Evidence | Type | What would verify it |
| --- | --- | --- | --- |
| Is there a public repository? | `curl` 404 for `github.com/jadmadi/swapaudit`; `gh repo list jadmadi` has no swapaudit; `proxy.golang.org` 404 | Observed | Public URL returns 200 and contains the source |
| Is the project in git? | `git status --short .` → `?? ./`; `git ls-tree -r HEAD` has no path; `git log -- active/swapaudit` empty | Observed | `git ls-files` includes `active/swapaudit/**`; commit history exists |
| Are the goal audit commits real? | `git cat-file -t fc6640d…` → "could not get object info"; same for two other SHAs in `goals/README.md` | Observed | SHAs resolve in the target repository |
| Does the build work? | `go test -race ./...` passes all packages; `go vet ./...` clean; `./swapaudit --version` works | Observed | Reproduce on a clean Linux host |
| Is the audit fast? | Timed local run: `elapsed_ms=81` with `--no-update-check` | Observed | Benchmark suite across distros and large `/tmp` |
| Is "zero dependencies" true? | `go.mod` has no `require` block; imports are standard library plus internal packages | Observed | `go list -deps` shows only stdlib/internal |
| Is PID-reuse protection implemented? | `internal/cleaner/cleaner.go:152-161` re-reads start ticks before `SIGTERM` | Observed | Unit/integration test with a reused PID fixture |
| Are file deletions confined? | `internal/cleaner/cleaner.go:75` checks `/tmp/`, `/var/tmp/`, `os.TempDir()`; `:83` rejects symlinks; `:88` checks UID | Observed | Adversarial path tests already exist in `cleaner_test.go` |
| Are process kills confined to the current user? | No UID check in `internal/cleaner/cleaner.go:148-161` | Observed (absence) | Read `/proc/PID/status` Uid and skip other users |
| Is orphan detection precise? | `internal/proc/proc.go:203-212` treats any parent named `systemd` or `init` as orphaned | Observed | Fixture: agent launched by a systemd user unit should not be cleanable |
| Are all advertised agent runtimes detected? | `internal/triage/triage.go:31-36` covers `agy`, `devin`, `chrome-devtools`, `antigravity`; no Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, Gemini | Observed (absence) | Fixtures for the top 5 runtimes |
| Is the update check truly background? | `internal/app/app.go:189-192` starts a 2 s context; `:252` cancels after 150 ms | Observed | Slow-server test; check whether state is written |
| Is "does not phone home" true? | Default run calls `api.github.com` unless `--no-update-check`, `SWAPAUDIT_NO_UPDATE_CHECK`, or `CI` is set; `GOVERNANCE.md:36` says it does not phone home | Observed / contradicted | Network capture on default run |
| Is the upgrade path real? | Code exists in `internal/updater/updater.go:368-470`; no release exists; ARM asset naming mismatch at `:409-413` vs `.goreleaser.yaml:43,51` | Observed / inferred | Public release + e2e upgrade on all four arches |
| Are releases signed? | No `cosign`, `minisign`, `gpg`, or signing step in `.goreleaser.yaml` or release workflow | Observed (absence) | Release includes a verifiable signature; updater rejects unsigned |
| Is there a package-manager distribution path? | README only documents raw download, clone, and `go install`; absence search found no AUR/Homebrew/Nix/deb/rpm/snap/flatpak config | Observed (absence) | Install on a clean distro without manual binary download |
| Is there a machine-readable output mode? | No `--json` flag in `internal/app/app.go`; only updater state JSON | Observed (absence) | `swapaudit --json` with a stable schema |
| Is there a revenue model? | No pricing, sponsor, donation, paid-support, subscription, or commercial-license references in product docs | Observed (absence) | Public funding/support page; first paid support contract |
| Is the README's kernel 3.10 requirement correct? | `README.md:151` says 3.10+; `internal/proc/proc.go:50` relies on `MemAvailable`, added in Linux 3.14 | Observed / inferred | Fixture `meminfo` without `MemAvailable`; document 3.14+ |
| Is `/var/tmp` cleaned? | `internal/cache/cache.go:37` scans only `/tmp`; `internal/cleaner/cleaner.go:75` would allow `/var/tmp` if it were scanned | Observed | Scan `/var/tmp` or correct the README |

---

## 3. Business model snapshot

| Dimension | Current evidence | Assessment |
| --- | --- | --- |
| Product / category | Linux CLI that attributes swap and tmpfs usage to processes/caches and safely reclaims it (`README.md:1-2`, `:47`) | Clear category: developer workstation memory hygiene |
| Economic buyer | None defined. Free source under WaqfDPL/AGPL; no pricing, sponsorship, support, or commercial tier (`README.md:338-354`) | **Absent by design or omission.** If a waqf endowment exists, its funding source is not in the repo |
| Daily user | Linux developer/sysadmin running AI coding agents, compilers, containers, and browsers (`README.md:29-47`; `GOVERNANCE.md:25-30`) | Plausible but inferred; no interviews or install data |
| Job to be done | Answer "what is in swap, what is in `/tmp`, and what can I safely reclaim right now?" (`README.md:40-47`) | Real, recurring, and currently solved with several free tools |
| Status quo / alternative | `ps_mem --swap`, `smem`, `earlyoom`, `systemd-oomd`, `systemd-tmpfiles`, BleachBit, manual `go clean`/Docker prune | Crowded with free, mature alternatives |
| Why now | More long-lived AI agent workers and build caches on developer workstations (`README.md:29-35`) | **Inferred.** The general pain is evidenced by mature tools; the agent-specific pain is not independently validated |
| Offer | Free static Linux binary, source, self-upgrade, dry-run, confirmation prompts, file/process safety guards | Strong local utility; public offer is not reachable |
| Revenue model | None. Dual license: WaqfDPL-Mulzim 1.0 and AGPL-3.0-or-later (`LICENSE`, `WaqfDPL-1.0.md`) | **No revenue mechanism.** Sustainability depends on the maintainer or an undocumented endowment |
| Pricing metric | Not applicable | No price, tier, entitlement, or billing code |
| Distribution | README: raw GitHub download, `git clone`, `go install`; no package manager; all documented paths currently 404 | **Broken.** This is the binding constraint |
| Moat | Classification heuristics, safety guards, terminal UX, agent-specific awareness | Weak and copyable; no data, network effect, or integration lock-in |
| Deployment / tenant model | Static Linux binary reading local `/proc`; no server, account, or tenant | Local-only; no multi-tenant isolation concerns. Distribution and endpoint trust replace tenancy risk |
| Locale | English-only UI and docs; no i18n or RTL code | Adequate for the developer-CLI wedge; demand outside English is untested |
| Stage | Pre-release/source-only. `CHANGELOG.md:7` has `0.1.0`, but no tag or release; local binary built from a parent-repo commit | **Prototype/demo, not beta or production** |

**Business model hypothesis (five lines, labeled):**

- **Product:** a Linux CLI that finds per-process swap, stale tmpfs caches, and dormant/orphaned agent processes, then reclaims them safely — **observed**.
- **Buyer:** none defined; the project is free software under a waqf/AGPL dual license — **observed**.
- **User:** a Linux developer or sysadmin running AI coding agents and heavy build workloads — **inferred** from the README and governance scope.
- **Job:** diagnose memory/swap/tmpfs pressure and reclaim space without killing active work — **inferred** from the flows and safety copy.
- **Money:** no revenue, pricing, sponsorship, or support mechanism — **observed**; whether an endowment funds it is **unknown**.

---

## 4. Offer and pricing audit

### Offer

- **What is included:** a static Linux binary, source, audit of swap/RAM/tmpfs, process classification, shell aggregation, dry-run, interactive confirmation, file cleanup, process termination, self-update check, and checksum-verified self-upgrade.
- **What is missing or promised but unbuilt:** a public repository, tags, releases, working download URLs, working `go install`, working `upgrade`, package-manager packages, signed artifacts, broad agent detection, machine-readable output, and a support/issue channel.
- **Table stakes vs differentiators vs distractions:**
  - **Table stakes:** accurate per-process swap, safe deletion, dry-run, clear docs, installable artifact, stable version.
  - **Differentiators:** unified swap + tmpfs + agent view; start-ticks PID-reuse protection; UID/symlink path guards; shell aggregation.
  - **Distractions:** none material in the code. The dual license is unusual but not a product distraction; the governance's mention of JSON output is an unbuilt expectation.
- **Self-serve vs sales-led:** self-serve only. There is no sales motion, no enterprise path, no support contract, and no hosted service.
- **Trial/free tier and end-of-trial behavior:** the entire product is free; there is no paid conversion boundary.
- **Enterprise path:** none. No SSO, audit export, DPA, SLA, or support offering. For a local CLI, most of those are not applicable, but support and signed releases are.

### Pricing

| Question | Evidence | Finding |
| --- | --- | --- |
| Pricing metric | No pricing or billing code; `README.md:338-354` | Not applicable; no revenue |
| Entry price | No price page or checkout | 0; free software |
| Limits / entitlements | No plan, quota, or entitlement checks | None |
| Billing implementation | No Stripe/Paddle/LemonSqueezy/subscription code; `go.mod` has no dependencies | Absent |
| Regional payment methods | N/A | N/A |
| Upgrade / expansion path | Software upgrade only; currently broken | Not a revenue expansion path |
| Downgrade / cancellation / refund | N/A | N/A |
| Unit economics | No cost or revenue data in repo | **Unknown.** Revenue is zero. Costs are maintainer time, GitHub Actions minutes, release storage, and domain/email; no figures are documented. Do not invent them |

**Unit-economics view:** This is not a SaaS. The relevant economics are maintainer time and distribution cost. The repo contains no funding, sponsorship, or cost data. If the project is a waqf endowment, the endowment source and budget are not documented. If it is a personal utility, that is a legitimate choice, but the README and governance should say so rather than implying a public product.

**Monetization options, without inventing demand:**

1. **Sponsorship/donations** for maintenance and release costs. Low friction, low expected revenue, compatible with the waqf framing.
2. **Paid support or priority fixes** for teams that adopt it. Requires a support boundary, response time, and a public issue tracker first.
3. **Commercial license or hosted team dashboard.** The AGPL alternative and WaqfDPL reciprocity make this legally and strategically complicated; legal review is required before promising it.
4. **Do nothing.** Treat it as a waqf/personal utility and remove public-product claims. This is the cheapest and most honest option if adoption is not a goal.

---

## 5. Claim-vs-capability audit

| Claim | Source | Implementation evidence | Status | Risk | Search scope for "Absent" |
| --- | --- | --- | --- | --- | --- |
| CI and Release badges | `README.md:3-4` | Badge URLs point to `github.com/jadmadi/swapaudit/actions/...`; repo is 404 | **Contradicted** | Trust: badges look active but lead nowhere | `curl` repo URL; `gh repo list jadmadi` |
| Download the latest static binary for amd64/arm64/armv7/riscv64 | `README.md:118-126` | No release exists; URL 404; `.goreleaser.yaml` can build them but has never published | **Contradicted** | Trust: first install step fails | `curl -I releases/latest`; `gh release list`; `git tag` |
| `git clone https://github.com/jadmadi/swapaudit.git` | `README.md:139` | Repo 404 | **Contradicted** | Trust: source unavailable | `curl` repo; `git ls-remote` |
| `go install github.com/jadmadi/swapaudit/cmd/swapaudit@latest` | `README.md:148` | Module proxy returns 404; repo 404 | **Contradicted** | Trust: Go users cannot install | `proxy.golang.org/.../@latest`; `go list -m` |
| `swapaudit upgrade` upgrades in place | `README.md:128-133`, `:276-281` | Code exists (`updater.go:368-470`), but no release; ARM asset name mismatch | **Contradicted / partial** | Trust: upgrade fails; ARMv7 would 404 even after release | `curl` release URL; compare asset templates |
| Zero dependencies | `README.md:47`, `:106` | `go.mod` has no `require`; stdlib only | **Verified** | None | `go list -deps` |
| Sub-second audit | `README.md:47`, `:107` | Local timed run 81 ms with `--no-update-check` | **Verified** (single host) | Low; not a benchmark | Run on 3 distros, 10k `/tmp` entries |
| PID-reuse protection via start ticks | `README.md:110-111` | `cleaner.go:152-161` re-reads start ticks before `SIGTERM` | **Verified** | Low | Existing unit tests; PID-reuse fixture |
| Strict path confinement to `/tmp` or `/var/tmp`, current UID | `README.md:112` | `cleaner.go:75` path prefix check; `:88` UID check; `cache.go:83` UID check | **Verified** for file cleanup | Low | Adversarial path tests exist |
| Symlink rejection at top-level and nested walk boundaries | `README.md:113` | `cache.go:78` skips top-level symlinks; `WalkDir` does not follow symlinks; `cleaner.go:83` rejects top-level symlink target | **Verified** | Low | `cache_test.go` symlink fixtures |
| 50 MB binary and 1 MB checksum stream limits | `README.md:114` | `updater.go:34-37`, `:507-535` | **Verified** | Low | `updater_test.go` oversized stream test |
| Daily background release check | `README.md:261-264` | `app.go:189-192` starts a 2 s check; `:252` cancels after 150 ms; state may not be written | **Partial** | Medium: update notices unreliable | Slow-server test; check state file |
| Precise detection of dormant AI coding agents | `README.md:108` | `triage.go:31-36` covers `agy`, `devin`, `chrome-devtools`, `antigravity` only | **Partial** | High: most agent users get no cleanup; promise overreach | Search for Claude/Codex/Cursor/Windsurf/Cline/Aider/OpenCode/Gemini in Go code |
| Orphaned agent detection "whose parent shell died" | `README.md:105`, CLI reference | `proc.go:203-212` treats any parent named `systemd`/`init` as orphaned, including legitimate systemd user units | **Contradicted / partial** | Critical: false-positive kill | Fixture with systemd-launched agent |
| "Conversations stay preserved on disk" | `cleaner.go:246` | No persistence check; the tool sends `SIGTERM` and does not verify agent state | **Unknown / unverified** | High: users may lose session state | Test with `agy`; document actual behavior |
| "100% offline-capable" and "does not phone home" | `GOVERNANCE.md:36` | Default run checks GitHub unless `--no-update-check`, env var, or `CI` | **Partial / contradicted** | High: privacy surprise and governance inconsistency | Network capture on default run |
| Cross-compilation for amd64/arm64/arm/riscv64 | `README.md:121`; `.github/workflows/ci.yml` | CI matrix and GoReleaser config exist; no public CI run or release | **Partial** | Medium: code can build, but the promise is unproven publicly | Run CI on a public repo; build all four targets |
| Kernel 3.10+ support | `README.md:151` | `proc.go:50` relies on `MemAvailable` (Linux 3.14+); no fallback | **Partial / contradicted** | Medium: RAM used is wrong on 3.10–3.13 | `meminfo` fixture without `MemAvailable` |
| Dual license WaqfDPL + AGPLv3 | `README.md:338-354`; `LICENSE`; `WaqfDPL-1.0.md`; `LICENSE-AGPL.txt` | Files exist and are consistent | **Verified** | Low/medium: unusual licensing may deter enterprise adoption | Legal review by counsel |

**Highest-risk claim gaps**

1. **Every install and upgrade path is 404** — the product is not publicly reachable. This is not a missing feature; it is a broken promise and a zero-adoption condition. Fix the repo and release before anything else.
2. **Orphan detection can kill a legitimate systemd-managed agent** — the README says "parent shell died," but the code treats systemd/init as proof of orphanhood. Remove the heuristic or require real parent-death evidence.
3. **"Conversations stay preserved on disk" is unverified** — a false reassurance before a destructive action. Verify it with the actual agent runtime or remove the claim.
4. **"Does not phone home" is false by default** — the update check is a network request. Make it opt-in, or amend the governance text to disclose it precisely.
5. **Agent coverage is far narrower than the README implies** — "AI coding agents" reads as a category; the code supports a handful of runtimes. Document the supported list and add configurable patterns.

---

## 6. Flow audit

### 6.1 Acquisition → activation

| Step | Implemented? | Evidence | Friction / missing state | Business impact |
| --- | --- | --- | --- | --- |
| Public landing or repo | No | `README.md:3-4`; repo 404 | No place to learn, star, watch, or download | Zero discoverability |
| Value proposition legible | Partial | `README.md:8-18`, `:29-47` | Strong README, but only visible if someone already has the source | No acquisition channel |
| Pricing / trial CTA | N/A | No price or trial | Free product; no CTA to a working artifact | No conversion path |
| Sign-up / sign-in | N/A | Local CLI | No account; not applicable | None |
| Workspace creation | N/A | Local CLI | No tenant/workspace model | None |
| Install | No public path | `README.md:124`, `:139`, `:148`; local `make build` works | Raw download, clone, and `go install` all fail; local build works | **First broken step** |
| First run | Yes | Timed run 81 ms; header + tables | Requires Linux and `/proc`; no onboarding tour | Strong local first value |
| First cleanup | Partial | `--dry-run`, confirmation prompts, guards; orphan/UID gaps | Safe for files; process kills need more guards | Trust risk on the destructive path |
| Invite team | N/A | No accounts | Not applicable | None |
| Upgrade | No | `updater.go:368-470`; no releases; ARM mismatch | Upgrade fails; no rollback instructions | Users stay on old or source builds |
| Support / feedback | No | No public issues; `SECURITY.md:16` points to a nonexistent repo | No way to report a bug or ask for help | No learning loop |

**First broken step:** public install. A user cannot get from the README to a binary.

**Time-to-value estimate:** local build and first run in minutes on a machine with Go 1.23+; public install is currently impossible. This is an estimate from the local build, not a measured onboarding funnel.

**Empty/loading/error coverage:** the CLI has clear empty states (`nothing to reclaim`), invalid-sort errors, dry-run output, and confirmation prompts. There is no loading state because the audit is synchronous and fast. Network update errors are swallowed or printed after the audit; they do not block the main flow.

### 6.2 Core delivery

Ticket intake, triage, promote-to-task, bridge, and customer-portal flows do not exist in this product. The equivalent core loop is **audit → select → dry-run → confirm → reclaim**, covered below. The portal section is replaced by CLI/operator experience in §6.4.

| Capability | Implemented? | Evidence | Gap | Impact |
| --- | --- | --- | --- | --- |
| Swap/RAM/tmpfs header | Yes | `internal/app/display.go:16-45`; `proc.go:32-70` | `/tmp` only for tmpfs; no `/var/tmp` display | Accurate enough for the stated job |
| Per-process `VmSwap` | Yes | `proc.go:73-100`; `app.go:61-103` | Only processes with `VmSwap > 0` appear | Correct focus |
| `/tmp` cache scan | Yes | `cache.go:37-145` | Only `/tmp`; no `/var/tmp`; keyword heuristics can include non-build caches | Cleanup can miss space or include surprises |
| Process classification | Partial | `triage.go:16-51` | Only a few agent runtimes; desktop daemon list is hardcoded | Core value is narrower than advertised |
| Shell aggregation | Yes | `triage.go:54-91` | Groups small bash/sh/zsh rows | Good UX |
| Safety guards | Partial | `cleaner.go:75-90` (files), `:148-161` (processes) | Files strong; process UID and orphan heuristics weak | Trust/data-loss risk |
| Dry-run and confirmation | Yes | `cleaner.go:31-53`, `:60-124` | Dry-run still prints `pruned`/`ended` with a prefix; clear enough | Good safety UX |
| Cleanup execution | Yes | `cleaner.go:200-275` | No audit log; no machine-readable result | Hard to verify what happened |
| Terminal formatting | Yes | `term.go`; `display.go` | `NO_COLOR`, `--ascii`, width adaptation verified | Good operator UX |
| JSON / machine output | No | No `--json` flag in `app.go` | Cannot feed dashboards or scripts | Limits automation and integrations |
| Export / retention / deletion | N/A | Local CLI; no stored user data | Not applicable | None |
| Privilege handling | Partial | No UID check for kills; no root warning | Root can act across users | Destructive risk |

### 6.3 Upgrade, trust, and money

| Area | Evidence | Finding |
| --- | --- | --- |
| Release tags / versioning | `git tag` empty; binary reports `1a8479e` | No semantic release; support and upgrade cannot identify a version |
| Release workflow | `.github/workflows/release.yml:1-4` triggers on `v*` tags; `.goreleaser.yaml` builds four arches | Config exists; no public repo, no tag, no run |
| CI | `.github/workflows/ci.yml` runs lint, race tests, and cross-compile | Config exists; no public run; local race tests pass |
| Asset naming | `.goreleaser.yaml:43,51` uses `.Arch` (= GOARCH); `updater.go:409-413` maps `arm` → `armv7` | ARMv7 asset mismatch; upgrade will 404 |
| Checksums | `.goreleaser.yaml:53-54`; `updater.go:432-444` | SHA256 verified; checksum fetched from the same release origin |
| Signatures / SBOM | No signing step; no SBOM config | Supply-chain trust gap |
| Security reporting | `SECURITY.md:16` says `jad@madi.me` or GitHub private reporting; `MAINTAINERS.md:13` says `contact@jadmadi.net`; repo 404 | Inconsistent contact; private reporting link is unusable |
| Issue tracker / support | No public repo or issues | No feedback loop; no support path |
| Funding / billing | No pricing, sponsor, donation, or support references | No revenue or sustainability mechanism |
| Telemetry / analytics | No analytics SDK; governance explicitly rejects cloud telemetry | Privacy-positive, but no measurement of activation or retention |
| Backups / migrations | Source is not in a standalone repo | No backup or history; this is the top operational risk |
| Incident process | No status page, runbook, or disclosure process beyond `SECURITY.md` | Trust gap for a tool that deletes files and kills processes |
| Compliance | Local `/proc` data only; update check exposes IP to GitHub | Minimal data footprint; no DPA needed for the local tool; disclose the update request |

### 6.4 CLI / operator experience (portal not applicable)

There is no portal, tenant, or customer account in this product. The portal-specific checks in the rubric do not apply; the equivalent surface is the CLI and the release/trust path.

| Question | Evidence | Finding |
| --- | --- | --- |
| Is help legible? | `app.go:121-124`, `display.go:300-360` | Yes: commands, flags, examples |
| Are errors actionable? | `app.go:139-146`, `:216-222` | Invalid sort returns exit 2 with a clear message; permission errors on cleanup are printed |
| Is destructive action previewable? | `--dry-run`; confirmation prompts | Yes; one of the strongest parts of the product |
| Is output accessible? | `NO_COLOR`, `--ascii`, width detection (`term.go:49-62`) | Yes |
| Is the result machine-readable? | No `--json` | No |
| Is there a version contract? | `--version` prints build metadata | No semantic version until tags exist |
| Is there an audit log? | Console output only | No persistent record of what was killed/deleted |
| Is there a safe default? | Cleanup flags default false; confirmation required | Yes |

### 6.5 Retention and revenue

| Step | Implemented? | Evidence | Gap | Business impact |
| --- | --- | --- | --- | --- |
| Return visit / recurring trigger | Partial / unknown | README workflows are manual; no `--watch`, timer example, or shell integration | Users must remember to run it; no habit loop | Low repeat usage; value is only realized during memory pressure |
| Upgrade / renewal | No | `upgrade` exists (`updater.go:368-470`) but no release; no paid renewal | Upgrade is broken; no commercial renewal motion | Users stay on source builds or abandon |
| Expansion | N/A | No seats, plans, usage tiers, or team features | No expansion mechanism | No revenue expansion |
| Referral | No | No public repo, community, or shareable artifact | No word-of-mouth loop | Acquisition depends on manual outreach |
| Revenue | No | No pricing, billing, sponsor, or support references | No revenue mechanism | Zero revenue; sustainability depends on the maintainer or an undocumented endowment |

**First broken step:** public repository and install. Without acquisition, retention and revenue cannot be observed. The current product is a one-shot utility with no recurring trigger; that is acceptable for a CLI, but it means retention must be measured by repeat use, not by account login.

---

## 7. Gap register

| ID | Category | Gap | Evidence | Type | Confidence | Severity | Affected flow | Business impact | Fix | Effort | Priority | Verification |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| G-01 | E. Acquisition / distribution | No public repository, tag, or release; all documented install/upgrade paths 404 | `README.md:3-4,124,139,148`; `git remote -v`; `git tag`; `gh repo list jadmadi`; curl 404; module proxy 404 | observed | High — direct URL and registry checks | Critical | Acquisition, activation, upgrade | Zero external installs; adoption and revenue are zero | Create a public standalone repo, import history, tag `v0.1.0`, publish four assets | M | Now | Repo 200; assets download; `go install` works; `upgrade` works |
| G-02 | L. Operations / support | Project is untracked inside the parent repo; goal audit SHAs do not exist | `git status --short .` → `?? ./`; `git ls-tree` no path; `git cat-file` fails for SHAs in `goals/README.md:42-51` | observed | High — direct git checks | Critical | Delivery, trust | No history, backup, review, or CI; audit claims ungrounded | Standalone repo; commit all files; re-run goal audits against real SHAs | S | Now | `git ls-files` includes source; SHAs resolve; CI green |
| G-03 | G. Core flows | Orphan heuristic treats any `systemd`/`init` parent as orphaned and can kill a legitimate systemd-managed agent | `proc.go:203-212`; `triage.go:31-36`; `cleaner.go:246` | observed | High — exact code path | Critical | Cleanup | False-positive kill destroys an agent session; trust incident | Require parent-death/session evidence; exclude systemd user units; add explicit `--include-managed` | M | Now | systemd-launched agent fixture is not cleanable; integration test |
| G-04 | G. Core flows | Process termination has no UID ownership check; root can signal other users' processes | `cleaner.go:148-161`; contrast file UID check at `cache.go:83` | observed | High — absence in kill path | Critical | Cleanup | Cross-user destructive action when run as root | Read UID from `/proc/PID/status`; skip other UIDs unless `--all-users` is explicit | S | Now | Other-UID fixture is skipped; root test logs skip |
| G-05 | D. Pricing / unit economics | No economic buyer, revenue, or documented sustainability mechanism | No pricing/sponsor/support references in product docs; `README.md:338-354`; license files | observed | Medium — intent may be a waqf endowment | High | Sustainability | No funding for releases, support, or maintenance | Decide: waqf endowment, sponsorship, paid support, or personal utility; publish it | M | Next | Funding page live; first sponsor/contract, or explicit personal-project statement |
| G-06 | E. Acquisition / distribution | Updater expects `swapaudit-linux-armv7`; GoReleaser emits `swapaudit-linux-arm` | `updater.go:409-413`; `.goreleaser.yaml:43,51`; GoReleaser template docs (`.Arch` = GOARCH, `.Arm` = GOARM) | observed / inferred | High — template semantics confirmed | High | Upgrade | ARMv7 upgrades 404 even after a release | Standardize asset names and updater mapping; add an asset-contract test | S | Now | Release asset list matches updater for all four arches; e2e upgrade test |
| G-07 | J. Trust / security | Default update check contradicts "does not phone home"; 150 ms cutoff cancels a 2 s check | `app.go:189-192,252`; `GOVERNANCE.md:36` | observed | High — code and docs conflict | High | First run, trust, updates | Privacy surprise; missed updates; governance inconsistency | Make update check opt-in, or amend governance and disclose the request; allow completion/persist state | S | Next | Default run makes no network call; slow-server test writes state |
| G-08 | J. Trust / security | Security/support contacts inconsistent; private reporting link points to a nonexistent repo; no public issues | `SECURITY.md:16`; `MAINTAINERS.md:13`; repo 404 | observed | High — direct text and URL check | High | Support, trust | Vulnerability reports and bug reports are lost | One canonical contact; public issue templates; fix security policy link | S | Now | Email and GitHub link work; issue template live |
| G-09 | J. Trust / security | No signed releases; checksum fetched from the same origin as the binary | `updater.go:432-444`; no cosign/minisign/gpg in `.goreleaser.yaml` or release workflow | observed | High — absence search | High | Upgrade, trust | GitHub/account compromise can serve a malicious update with a matching checksum | Sign artifacts (cosign/minisign), publish SBOM, verify signature in updater | M | Next | Signature present and verified; unsigned artifact rejected |
| G-10 | B. Value proposition | Agent detection only covers `agy`, `devin`, `chrome-devtools`, `antigravity`; README implies broad AI-agent support | `triage.go:31-36`; `README.md:108` | observed | High — exact list | High | Core value | Most users of Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, or Gemini get no agent cleanup | Configurable patterns + process-tree detection; support top 5 runtimes; document the list | M | Next | Fixtures for 5+ runtimes; docs match code |
| G-11 | C. Offer / scope | "Conversations stay preserved on disk" is unverified before a destructive action | `cleaner.go:246` | observed / unknown | Medium — behavior not tested | High | Cleanup | Users may lose session state on a false assumption | Verify with the actual agent runtime or remove the claim | S | Now | Test with `agy`; docs updated |
| G-12 | C. Offer / scope | Cleaner allows `/var/tmp`, but scanner only scans `/tmp`; README implies both | `cache.go:37`; `cleaner.go:75`; `README.md:112` | observed | High — exact code path | Medium | Cleanup | Incomplete cleanup; claim mismatch | Scan `/var/tmp` or correct the README | S | Later | `/var/tmp` fixture is scanned/cleaned, or docs corrected |
| G-13 | C. Offer / scope | README claims kernel 3.10+; `MemAvailable` requires 3.14+, so RAM used is wrong on 3.10–3.13 | `README.md:151`; `proc.go:50`; earlyoom docs (MemAvailable requires 3.14+) | observed / inferred | High — code plus kernel fact | Medium | First run | Wrong memory diagnosis on older kernels | Require 3.14+ or add a fallback (`MemFree + Buffers + Cached`) | S | Next | `meminfo` fixture without `MemAvailable` |
| G-14 | G. Core flows | `/proc/PID/sched` may be restricted by permissions/hidepid; dormant detection silently returns `-1` | `proc.go:217-218`; `app.go:71` | observed / inferred | Medium — depends on system policy | Medium | Core value | Dormant cleanup silently misses processes on hardened systems | Fallback to `/proc/PID/stat` or process start time; emit a warning | M | Next | hidepid/permission-denied test |
| G-15 | E. Acquisition / distribution | No package-manager distribution or install script; only raw download/source | `README.md:118-148`; absence search for AUR/Homebrew/Nix/deb/rpm/snap/flatpak | observed | High — absence search | High | Acquisition | High install friction; low adoption | Add AUR, Homebrew, Nix, deb/rpm, or a checksum-verified install script | M | Next | Clean-container install on two distros |
| G-16 | I. CLI / automation | No `--json` output; governance mentions JSON schemas that do not exist | `app.go:121-124`; no `--json` flag; `GOVERNANCE.md:51` | observed | High — flag search | Medium | Automation, integrations | Cannot feed scripts, dashboards, or CI | Stable versioned JSON schema + tests | M | Next | `jq` can parse `swapaudit --json`; schema test |
| G-17 | M. Measurement | No usage, installation, or feedback measurement | No analytics/telemetry; no public releases or issues | observed | High — absence search | Medium | Learning | Cannot measure activation, retention, or demand | GitHub download counts, issue templates, 5 interviews; optional local-only diagnostics | S | Next | Weekly dashboard; 5 interviews completed |
| G-18 | J. Trust / security | UPX compression enabled; no reproducible-build check or uncompressed variant | `.goreleaser.yaml:30-36` | observed / inferred | Medium — scanner behavior not tested | Medium | Distribution, trust | Endpoint tooling may flag packed binaries; reproducibility harder | Publish an uncompressed variant; add a reproducible-build check | S | Later | Rebuild checksum matches; variant published |
| G-19 | K. Retention | One-shot CLI has no recurring trigger, timer example, or habit loop | README workflows; no watch/timer/integration | observed | Medium — absence search | Medium | Retention | Users forget to run it; low repeat usage | Document a systemd timer/cron example with `--json`; do not add a daemon | S | Later | Example timer; repeat-usage interviews |
| G-20 | A. Problem / market / ICP | Demand and ICP are unvalidated; no interviews, install data, or issue reports | No releases, telemetry, or issues; "why now" is inferred from the README | unknown | Low — no demand evidence either way | High | Learning | May optimize for a user who does not exist | 5 user interviews + public release install test | S | Now | 5 interviews; 10 external installs |

**Gap count by category**

| Category | Critical | High | Medium | Low |
| --- | --- | --- | --- | --- |
| A. Problem / market / ICP | 0 | 1 | 0 | 0 |
| B. Value proposition | 0 | 1 | 0 | 0 |
| C. Offer / scope | 0 | 1 | 2 | 0 |
| D. Pricing / unit economics | 0 | 1 | 0 | 0 |
| E. Acquisition / distribution | 1 | 2 | 0 | 0 |
| F. Activation / onboarding | 0 | 0 | 0 | 0 |
| G. Core flows | 2 | 0 | 2 | 0 |
| H. Service workflow / bridge | 0 | 0 | 0 | 0 |
| I. Portal / customer experience | 0 | 0 | 1 | 0 |
| J. Trust / security / compliance | 0 | 3 | 1 | 0 |
| K. Retention / expansion | 0 | 0 | 1 | 0 |
| L. Operations / support | 1 | 0 | 0 | 0 |
| M. Measurement | 0 | 0 | 1 | 0 |
| N. Localization / market fit | 0 | 0 | 0 | 0 |
| O. Scalability / cost | 0 | 0 | 0 | 0 |
| P. Moat / roadmap | 0 | 0 | 0 | 0 |
| **Total** | **4** | **9** | **7** | **0** |

**Why categories F, H, N, O, and P are empty or near-empty:**

- **F (activation):** local build and first run work; the activation problem is distribution (G-01), not onboarding.
- **H (service workflow/bridge):** this product has no ticket-to-task bridge or service workflow.
- **N (localization):** English-only is a deliberate scope for a developer CLI; no evidence of demand for Arabic/RTL. Deferred, not a current gap.
- **O (scalability/cost):** no server or multi-tenant cost base. The relevant scale risk is large `/tmp` scans, which is unbenchmarked but not yet a business blocker.
- **P (moat):** weak moat is real but overlaps B (differentiation) and D (sustainability); no separate row to avoid double counting.

---

## 8. Market and competitor research

| Competitor / alternative | Segment | Offer | Pricing metric / entry | Strengths | Gap or opportunity | Source |
| --- | --- | --- | --- | --- | --- | --- |
| **ps_mem** | Linux developers/sysadmins | Per-process private/shared memory; `--swap` flag | Free; LGPL-2.1; pip and distro packages | Simple, accurate per-process RAM; explicit `--swap` | No tmpfs scan, no agent-aware cleanup, no safety guards | [ps_mem GitHub](https://github.com/pixelb/ps_mem), accessed 2026-10-19 |
| **smem** | Linux developers/sysadmins | PSS/USS per process, user, mapping; snapshots and charts | Free; GPLv2+; distro package | Mature PSS reporting; `smemcap` for embedded/snapshot analysis | Man page explicitly says USS/PSS do **not** include swapped-out memory; no cleanup | [smem man page](https://manpages.debian.org/bookworm/smem/smem.8.en.html), accessed 2026-10-19; [smem site](https://www.selenic.com/smem/), accessed 2026-10-19 |
| **earlyoom** | Linux desktops/servers | Daemon kills the largest process when RAM and swap are low | Free; MIT; apt/dnf/pacman packages | Prevents system lockups; mature, low overhead (~2 MiB RSS); `--prefer`/`--avoid` | Reactive kill, no attribution, no targeted cache cleanup, no audit | [earlyoom GitHub](https://github.com/rfjakob/earlyoom), accessed 2026-10-19 |
| **systemd-oomd** | systemd distros | PSI/cgroup-based OOM killer | Free; bundled with systemd | OS-integrated, cgroup-aware, PSI-based | Kills whole cgroups; no per-process swap attribution or cleanup decision | [systemd-oomd man page](https://www.freedesktop.org/software/systemd/man/latest/systemd-oomd.service.html), accessed 2026-10-19 |
| **systemd-tmpfiles** | systemd distros | Age-based cleanup of `/tmp` and `/var/tmp` | Free; bundled with systemd | Default OS mechanism; policy-driven; `--dry-run` | No process/swap awareness, no agent classification, no interactive preview | [systemd-tmpfiles man page](https://www.man7.org/linux/man-pages/man8/systemd-tmpfiles.8.html), accessed 2026-10-19 |
| **BleachBit** | Desktop users/sysadmins | GUI/CLI cache cleaner, secure deletion | Free; GPLv3; "No subscriptions"; "100% Free Forever" | Broad application cleaners; preview before deletion; offline | No swap attribution or agent-process cleanup | [BleachBit](https://www.bleachbit.org/), accessed 2026-10-19 |
| **Stacer** | Desktop users | GUI system optimizer/monitor | Free; GPL-3.0; project abandoned | GUI monitoring and cleanup; 9.3k stars | Abandoned; no swap attribution or agent awareness | [Stacer GitHub](https://github.com/oguzhaninan/Stacer), accessed 2026-10-19 |
| **Status quo: `go clean`, Docker prune, manual `find`/`ps`** | Developers | Per-tool cache cleanup and manual process inspection | Free; bundled with Go/Docker/coreutils | Precise per-tool commands; no extra binary | Fragmented; no unified swap/tmpfs view; no safety model for agent processes | [Go clean docs](https://pkg.go.dev/cmd/go#hdr-Remove_object_files_and_cached_files), accessed 2026-10-19; [Docker prune docs](https://docs.docker.com/engine/manage-resources/pruning/), accessed 2026-10-19 |

**What buyers expect in this category**

- **Frictionless install:** package-manager availability is the norm. earlyoom lists apt, dnf, and pacman packages; BleachBit ships distro packages. Raw GitHub downloads are acceptable for early adopters but not for mainstream developer adoption.
- **Preview before deletion:** BleachBit explicitly shows every file before deletion; `systemd-tmpfiles` offers `--dry-run`; swapaudit's `--dry-run` and confirmation prompts meet this expectation.
- **Low overhead and no daemon:** earlyoom uses about 2 MiB RSS; `systemd-oomd` is native. swapaudit's 81 ms one-shot run and 6.6 MB binary are competitive.
- **Privacy/offline:** BleachBit says all features work offline; swapaudit's governance says it does not phone home. The default update check violates that expectation unless disclosed or opt-in.
- **Clear license and support:** all direct substitutes are free/open source. A paid product needs a support or hosted-value wedge that these tools do not provide.
- **Safety and auditability:** for tools that delete files or kill processes, users expect dry-run, confirmation, and a record of what happened. swapaudit has dry-run and confirmation; it lacks a persistent audit log.

**Differentiation assessment**

- **Credible wedge:** swapaudit combines per-process `VmSwap`, tmpfs cache discovery, and agent-aware safe cleanup in one fast pass. `smem` explicitly excludes swapped memory; `ps_mem` has `--swap` but no cleanup; `systemd-tmpfiles` cleans files but has no process awareness. That combination is real.
- **Where it is at parity:** process memory reporting (`ps_mem`), dry-run/preview (BleachBit, `systemd-tmpfiles`), free/open-source distribution.
- **Where it is behind:** public distribution, package-manager presence, signed releases, broad agent coverage, machine-readable output, support channel, and reproducible builds.
- **What the wedge requires to be defensible:** (1) a public, signed release; (2) verified safety with no false-positive kills; (3) support for the agent runtimes target users actually run; (4) distribution through the channels developers use. Without those, the wedge is a local script with a good README.

**Market-size and willingness-to-pay evidence:** **unknown.** No market sizing was attempted, and no primary source was found for willingness to pay for a standalone Linux workstation memory cleaner. The fact that every direct substitute is free or OS-bundled is the strongest available signal: a per-seat subscription is unlikely without a support or team-level value proposition.

---

## 9. Structured thinking analysis

### 9.1 Issue tree

```text
Question: Where does swapaudit lose value between source code and sustained use?
Splitting rule: customer lifecycle stage, then evidence type

Distribution
  ├─ Is there a public repo/release?          [observed: 404, no tags, no releases]
  ├─ Is there a package-manager path?         [observed: absent]
  └─ Do the install docs work?                [observed: no; local build works]

Activation
  ├─ Can a user build from source?            [observed: yes, Go 1.23+]
  ├─ Is first run fast?                       [observed: 81 ms, one host]
  └─ Is first cleanup safe?                   [partial: file guards strong; process gaps]

Core value
  ├─ Per-process swap attribution             [observed: yes]
  ├─ tmpfs cache discovery                    [observed: /tmp only]
  ├─ Agent-aware classification               [partial: 3–4 runtimes]
  └─ Machine-readable output                  [observed: absent]

Trust
  ├─ Signed releases                          [observed: absent]
  ├─ Security/support contact                 [observed: inconsistent]
  ├─ No phone home                            [contradicted by default update check]
  └─ No false-positive kills                  [unknown/at risk: orphan heuristic]

Retention
  ├─ Recurring trigger or timer               [unknown: no example]
  ├─ Upgrade path                             [observed: broken]
  └─ Reason to return next week               [inferred: only when memory pressure recurs]

Sustainability
  ├─ Economic buyer                           [observed: none]
  ├─ Funding source                           [unknown]
  └─ Moat                                     [inferred: weak, copyable heuristics]

Highest-leverage leaf: Distribution → public repo/release.
Why: it is the cheapest test of demand and the precondition for observing every other leaf.
```

### 9.2 Inversion

**Outcome to avoid:** "In 12 months, no external user has installed swapaudit, or a user loses work after a false-positive kill, or the maintainer abandons the project."

| Failure mechanism | Evidence | Prevention / detection / recovery | Verification |
| --- | --- | --- | --- |
| Never publish a repo or release | `README.md:124,139,148`; repo 404; no tags | Prevention: publish standalone repo + `v0.1.0`. Detection: release download counts. Recovery: none needed if caught early | Public URLs 200; 10 installs |
| Release assets do not match the updater | `updater.go:409-413`; `.goreleaser.yaml:43,51` | Prevention: asset-name contract test for all four arches. Detection: e2e upgrade CI. Recovery: manual install instructions | Assets match; e2e upgrade passes |
| Orphan heuristic kills a systemd-managed agent | `proc.go:203-212`; `cleaner.go:246` | Prevention: require parent-death evidence; exclude systemd user units. Detection: dry-run diff and audit log. Recovery: none for lost session state; document and apologize | systemd fixture not cleanable |
| Root run signals another user's process | `cleaner.go:148-161` | Prevention: UID check. Detection: log skipped UIDs. Recovery: incident review | Other-UID fixture skipped |
| Unsigned update is replaced by a malicious binary | `updater.go:432-444`; no signing config | Prevention: sign releases and verify in updater. Detection: signature failure. Recovery: revoke release, rotate key, notify users | Tampered binary rejected |
| Maintainer burns out with no funding | No pricing/sponsor/support references | Prevention: choose a sustainability model. Detection: issue response time and release cadence. Recovery: archive or hand off | Funding page; response-time metric |
| Agent coverage stays narrow and users churn | `triage.go:31-36` | Prevention: configurable patterns and process-tree detection. Detection: issue reports. Recovery: add patterns quickly | 5-agent fixtures; issue SLA |
| Update check privacy surprise damages trust | `app.go:189-192`; `GOVERNANCE.md:36` | Prevention: opt-in or disclose. Detection: user feedback. Recovery: change default and document | Network capture shows no default call |
| `/tmp` scan misses `/var/tmp` or large trees | `cache.go:37`; `README.md:112` | Prevention: scan both or correct docs. Detection: user reports. Recovery: add path | `/var/tmp` fixture |
| Kernel 3.10–3.13 shows wrong RAM used | `proc.go:50`; `README.md:151` | Prevention: require 3.14+ or fallback. Detection: fixture. Recovery: docs and patch | `meminfo` fixture |

**Most likely failure:** the project is never published, so no user can install it.

**Most preventable high-impact failure:** a false-positive process kill from the orphan heuristic or missing UID check. It is cheap to fix and irreversible in user trust.

### 9.3 Impact-effort

**Impact definition:** installability (can a user get and run it?), trust/data-loss (can it destroy work?), adoption (does it reach target users?), sustainability (can it be maintained?), and learning (does it resolve a major unknown?).

**Effort definition:** code, release engineering, operations, communications, and compliance/legal work; not elapsed time.

| Gap | Impact (why) | Effort (why) | Priority |
| --- | --- | --- | --- |
| G-01 public repo/release | High — without it there are no users, feedback, or revenue | Medium — repo setup, import, tag, release, asset checks | Now |
| G-02 standalone git history | High — no backup, review, or CI; audit claims ungrounded | Low — initialize repo, commit, re-run audits | Now |
| G-03 orphan heuristic | High — irreversible session loss and trust damage | Medium — parent-death logic, systemd exclusions, tests | Now |
| G-04 process UID check | High — cross-user kill risk as root | Low — read UID, skip other users, test | Now |
| G-06 ARM asset naming | High — upgrade fails on one of four advertised arches | Low — rename templates and updater mapping, test | Now |
| G-08 security/support contact | High — vulnerability reports lost; no feedback | Low — one contact, issue templates, fix link | Now |
| G-11 "conversations preserved" | High — false reassurance before destructive action | Low — test or remove claim | Now |
| G-07 update check privacy/reliability | High — governance inconsistency and missed updates | Low — opt-in flag or text change; state fix | Next |
| G-09 signed releases | High — supply-chain trust | Medium — signing key, workflow, updater verification | Next |
| G-10 agent coverage | High — core value only works for a few runtimes | Medium — configurable patterns, process trees, fixtures | Next |
| G-15 package managers | High — adoption friction | Medium — AUR/Homebrew/Nix/deb/rpm or install script | Next |
| G-16 JSON output | Medium — automation and integrations | Medium — schema, flag, tests | Next |
| G-17 measurement | Medium — cannot learn from users | Low — release counts, issues, interviews | Next |
| G-05 sustainability | High — long-term maintenance | Medium — decision plus public funding/support page | Next |
| G-20 demand validation | High — determines whether the product should exist | Low — 5 interviews and a release test | Now |
| G-12 `/var/tmp` mismatch | Medium — incomplete cleanup | Low — scan or correct docs | Later |
| G-13 kernel 3.10 claim | Medium — wrong diagnosis on old kernels | Low — fallback or docs | Next |
| G-14 `/proc/sched` permissions | Medium — silent feature failure | Medium — fallback and warning | Next |
| G-18 UPX/reproducibility | Medium — endpoint/scanner friction | Low — uncompressed variant, build check | Later |
| G-19 retention | Medium — users forget a one-shot tool | Low — timer example and `--json` | Later |

**Top three moves:** publish the repo/release, fix the destructive process heuristics, and make the trust story coherent (update check, security contact, signed releases).

### 9.4 Optional methods used

- **Ladder of Inference:** used to check the leap from "the public URL is 404" to "there are no users." Observable data: 404s, no tags, no releases, no public repo. Interpretation: no public distribution. Assumption: no private distribution channel. Conclusion: no public installable user base. Action: publish and measure. This keeps the report from claiming that no one uses the tool privately.
- **Second-order thinking:** used to evaluate broadening agent detection. First-order: more runtimes supported, more users served. Second-order: more false positives, more destructive risk, more support load. Third-order: a single bad kill could define the product's reputation. The design implication is configurable patterns, dry-run defaults, an audit log, and opt-in destructive actions — not simply adding names to `triage.go`.

The mandatory methods were **Issue Tree**, **Inversion**, **Impact-Effort**, and **Minto Pyramid**. The report leads with the verdict and groups the argument into distribution, trust/safety, money/differentiation, then evidence.

---

## 10. Prioritized roadmap

### Now (next 1–2 delivery cycles)

| Move | Gap(s) | Why now | Acceptance check |
| --- | --- | --- | --- |
| Create a standalone public repo and publish `v0.1.0` with four architecture assets | G-01, G-02, G-06 | Nothing else matters until a user can install; this is also the cheapest demand test | Repo 200; assets download; `go install` works; `upgrade` succeeds on amd64/arm64/armv7/riscv64 |
| Fix destructive process safety: orphan evidence, UID check, and the "conversations preserved" claim | G-03, G-04, G-11 | One false-positive kill is irreversible and defines trust | systemd-managed agent fixture is not cleanable; other-UID fixture skipped; claim verified or removed |
| Make the trust surface coherent: canonical security contact, public issues, opt-in or disclosed update check | G-07, G-08 | Users need a way to report bugs and understand network behavior | Email/GitHub link works; default run makes no network call or docs disclose it; issue template live |
| Run 5 target-user interviews and a public install test | G-20 | Determines whether the agent-aware wedge is real before more code | 5 interviews; 10 external installs; 3 pain reports |

### Next (after the first validation)

| Move | Gap(s) | Trigger to start | Acceptance check |
| --- | --- | --- | --- |
| Sign releases and verify signatures in the updater | G-09 | First public release exists | Signature verified; tampered binary rejected |
| Add package-manager distribution and a checksum-verified install script | G-15 | At least 10 installs from raw download or user requests | Clean-container install on two distros |
| Add configurable agent patterns/process-tree detection | G-10 | Interviews show users run agents outside the current list | Fixtures for 5+ runtimes; docs match |
| Add `--json` with a versioned schema | G-16 | A user asks to automate or dashboard the audit | `jq` can parse `swapaudit --json`; schema test |
| Choose and publish a sustainability model | G-05 | Repo is public and support requests begin | Funding/support page live; first sponsor/contract or explicit personal-project statement |
| Add release/install measurement | G-17 | First release exists | Weekly download/install/issue dashboard |

### Later (defer until traction or a decision)

| Move | Gap(s) | Why later |
| --- | --- | --- |
| Scan `/var/tmp` or correct the README | G-12 | `/tmp` tmpfs is the primary pain; `/var/tmp` can wait for a user report |
| Add kernel fallback for 3.10–3.13 | G-13 | Modern developer workstations are 5.x/6.x; document the requirement first |
| Add `/proc/sched` fallback and warning | G-14 | Hardened systems are a smaller segment; the fallback needs design |
| Publish an uncompressed binary and reproducible-build check | G-18 | Enterprise/endpoint feedback will tell you whether UPX is a problem |
| Document a systemd timer/cron example | G-19 | Only useful after `--json` and a stable release exist |

### Not worth doing

| Idea | Why not |
| --- | --- |
| Hosted SaaS, account system, or cloud telemetry | Violates the governance anti-goals and the privacy posture; no buyer evidence |
| GUI or Electron app | Governance anti-goal; free GUI competitors (BleachBit, Stacer) already exist and Stacer is abandoned |
| Windows/macOS ports | The product depends on Linux `/proc`; out of scope and would dilute the wedge |
| Background daemon | Governance anti-goal; earlyoom/systemd-oomd already own that job |
| Paid per-seat tier without a support/team value proposition | Every direct substitute is free or OS-bundled; no willingness-to-pay evidence |
| Feature breadth for its own sake | The binding constraint is distribution and trust, not feature count |

---

## 11. Experiments to resolve uncertainty

| # | Hypothesis | Cheapest test | Success metric | Cost | Time-box | What it changes |
| --- | --- | --- | --- | --- | --- | --- |
| E-01 | External developers will install a public release | Publish repo + `v0.1.0`, announce in one Linux/Go community | 10 external installs and 3 issue reports | Release work | 2 weeks | If <10, demand is weak; if ≥10, proceed to package managers |
| E-02 | Linux developers running AI agents feel the swap/tmpfs pain frequently | 5 structured interviews with target users | 3 of 5 describe the pain and a current workaround | Interview time | 1 week | Validates or kills the "why now" thesis |
| E-03 | swapaudit finds meaningful reclaimable memory without false kills | Concierge cleanup with 3 users, dry-run first | ≥1 GB reclaimable per user; zero false positives | 3 sessions | 1 week | Validates core value and safety |
| E-04 | Package-manager distribution increases activation | Add one channel (AUR or Homebrew) and compare installs | Install conversion or 10 installs from the channel | Packaging | 2 weeks | Decides whether to invest in more channels |
| E-05 | Users care about signed releases | Add signing and publish a verification note | Download/verification feedback; no signature failures | Signing setup | 1 week | If no interest, keep checksums but document the trust model |
| E-06 | Teams will pay for support or priority fixes | Ask 10 users; offer a paid support pilot | 2 expressions of interest | Outreach | 2 weeks | If zero, keep the project waqf/personal and stop treating revenue as a near-term goal |

Prefer E-01 and E-02 first. They are cheap and decide whether the rest of the roadmap is worth building.

---

## 12. Open questions

| Question | Why it matters | Who can answer it | How |
| --- | --- | --- | --- |
| Is `jadmadi/swapaudit` intentionally private, or does it not exist? | All 404 findings depend on this | Maintainer | Inspect the GitHub account/repo settings; confirm the intended URL |
| Is this a business, a waqf endowment, or a personal utility? | Determines whether "no revenue" is a defect or a design choice | Maintainer | Written statement in README/GOVERNANCE |
| Who is the target user, and which agent runtimes do they actually run? | Drives agent coverage and the wedge | 5 target users | Interviews |
| What is the acceptable false-positive rate for process termination? | A single bad kill may be unacceptable | Maintainer and users | Define policy; test with fixtures |
| Are users on kernels <3.14 or systems with `hidepid`? | Determines whether G-13/G-14 are real | Users | Support questions and issue reports |
| Which distribution channels do target users prefer? | AUR/Homebrew/Nix/deb/rpm/install script | Users | Interview and package-manager telemetry |
| Is there legal review of the WaqfDPL + AGPL dual license? | Enterprise adoption and commercial licensing depend on it | Counsel | Legal review |
| What support/security response commitment can the maintainer make? | Required for a public issue tracker and security policy | Maintainer | Publish a support policy |

---

## Appendix A — Source log

| # | Source | URL | Accessed | Type | Claim used |
| --- | --- | --- | --- | --- | --- |
| S-01 | smem man page (Debian) | https://manpages.debian.org/bookworm/smem/smem.8.en.html | 2026-10-19 | primary | smem reports PSS/USS and explicitly excludes swapped-out memory |
| S-02 | smem project page | https://www.selenic.com/smem/ | 2026-10-19 | primary | smem features: per-process/user/mapping reports, snapshots, charts |
| S-03 | ps_mem GitHub | https://github.com/pixelb/ps_mem | 2026-10-19 | primary | `--swap` flag; LGPL-2.1; simple per-process memory reporting |
| S-04 | earlyoom GitHub | https://github.com/rfjakob/earlyoom | 2026-10-19 | primary | MIT; daemon kills largest process when RAM/swap low; distro packages; `MemAvailable` requires kernel 3.14+ |
| S-05 | systemd-oomd man page | https://www.freedesktop.org/software/systemd/man/latest/systemd-oomd.service.html | 2026-10-19 | primary | PSI/cgroup-based OOM killer; kills cgroups |
| S-06 | systemd-tmpfiles man page | https://www.man7.org/linux/man-pages/man8/systemd-tmpfiles.8.html | 2026-10-19 | primary | Age-based cleanup of `/tmp` and `/var/tmp`; `--dry-run` |
| S-07 | BleachBit | https://www.bleachbit.org/ | 2026-10-19 | primary | GPLv3; no subscriptions; preview before deletion; offline |
| S-08 | Stacer GitHub | https://github.com/oguzhaninan/Stacer | 2026-10-19 | primary | GPL-3.0; GUI optimizer; project marked abandoned |
| S-09 | Go `clean` docs | https://pkg.go.dev/cmd/go#hdr-Remove_object_files_and_cached_files | 2026-10-19 | primary | `go clean -cache` removes the Go build cache |
| S-10 | Docker prune docs | https://docs.docker.com/engine/manage-resources/pruning/ | 2026-10-19 | primary | `docker system prune` cleans unused objects and build cache |
| S-11 | GoReleaser templates | https://goreleaser.com/customization/general/templates/ | 2026-10-19 | primary | `.Arch` = GOARCH; `.Arm` = GOARM; `.Arch` alone does not encode ARMv7 |
| S-12 | GitHub repository check | https://github.com/jadmadi/swapaudit | 2026-10-19 | primary | HTTP 404; no public repo/releases |
| S-13 | Go module proxy check | https://proxy.golang.org/github.com/jadmadi/swapaudit/@latest | 2026-10-19 | primary | HTTP 404; `go install` path is unavailable |

No market-size or competitor-price numbers were invented. All direct competitor offers above are free or OS-bundled; no paid tier was observed on the cited pages.

---

## Appendix B — File evidence

| File | Lines | What it proves |
| --- | --- | --- |
| `README.md` | 3–4 | CI and Release badges point to `jadmadi/swapaudit`, which is 404 |
| `README.md` | 47, 106–115 | Zero dependencies, sub-second audit, safety claims |
| `README.md` | 118–148 | Install methods: raw download, clone, `go install` |
| `README.md` | 151 | Kernel 3.10+ requirement |
| `README.md` | 259–281 | Daily update check and in-place upgrade claims |
| `README.md` | 338–354 | Dual license WaqfDPL + AGPL |
| `internal/app/app.go` | 121–124 | Only `help` and `upgrade` commands |
| `internal/app/app.go` | 189–192, 252 | Update check goroutine and 150 ms cutoff |
| `internal/app/app.go` | 227–241 | Cleanup flag consolidation and quick commands |
| `internal/app/display.go` | 16–45, 300–360 | Header/table rendering and help text |
| `internal/cleaner/cleaner.go` | 75, 83, 88 | File path confinement, symlink rejection, UID check |
| `internal/cleaner/cleaner.go` | 148–161 | Process kill guards: PID > 1, self, start-ticks; **no UID check** |
| `internal/cleaner/cleaner.go` | 246 | "Conversations stay preserved on disk" prompt |
| `internal/proc/proc.go` | 50 | `MemAvailable` dependency |
| `internal/proc/proc.go` | 203–212 | Orphan heuristic treats `systemd`/`init` parent as orphaned |
| `internal/proc/proc.go` | 217–218 | Reads `/proc/PID/sched` for idle time |
| `internal/triage/triage.go` | 31–36 | Supported agent runtimes: `agy`, `devin`, `chrome-devtools`, `antigravity` |
| `internal/cache/cache.go` | 17, 37 | Default patterns and `/tmp`-only scan |
| `internal/cache/cache.go` | 78, 83, 120, 140 | Symlink/UID guards, recursive mtime, cleanable threshold |
| `internal/updater/updater.go` | 24–37 | Default repo, 24 h interval, 50 MB/1 MB limits |
| `internal/updater/updater.go` | 368–470 | Upgrade flow, checksum verification, atomic rename |
| `internal/updater/updater.go` | 409–413 | ARM → `armv7` asset name mapping |
| `.goreleaser.yaml` | 13–18 | Build matrix: amd64, arm64, arm, riscv64; `goarm: 7` |
| `.goreleaser.yaml` | 30–36 | UPX enabled for amd64/arm64 |
| `.goreleaser.yaml` | 43, 51 | Archive and binary name templates use `.Arch`, not `.Arm` |
| `.goreleaser.yaml` | 53–54 | Checksums file, no signing |
| `.github/workflows/ci.yml` | 1–4, 25–48 | CI runs on `main`; race tests and cross-compile matrix |
| `.github/workflows/release.yml` | 1–4 | Release triggers on `v*` tags |
| `go.mod` | 1–3 | Module path; no `require` dependencies |
| `CHANGELOG.md` | 7–9 | `0.1.0` entry, no matching tag |
| `SECURITY.md` | 16 | `jad@madi.me` and GitHub private reporting |
| `MAINTAINERS.md` | 13 | `contact@jadmadi.net` |
| `GOVERNANCE.md` | 30, 36, 51 | Under-200 ms goal; no phone home; JSON schema mention |
| `goals/README.md` | 42–51 | Ten "verified" goals with commit SHAs that do not resolve |
| `internal/updater/updater_test.go` | — | 46.4% package coverage; core upgrade path under-tested |
| Local commands | — | `go test -race ./...` pass; `go vet` clean; audit 81 ms; `--version` works |

---

## Appendix C — Search scope

| Finding | Paths / globs | Keywords | Result |
| --- | --- | --- | --- |
| No public repo/release | `README.md`, `go.mod`, `.github/**`, `internal/updater/updater.go`; shell `git remote -v`, `git tag`, `gh repo list`, `curl`, `proxy.golang.org` | `jadmadi/swapaudit`, `releases/latest`, `go install` | 404; no tags; no repo in `gh repo list`; module proxy 404 |
| No package-manager distribution | `**/*.{md,yml,yaml,sh}` excluding `goals/` | `aur`, `homebrew`, `nixpkgs`, `deb`, `rpm`, `snap`, `flatpak`, `scoop`, `winget`, `install.sh` | No distribution config; only false positives "Tauri" and "snapshot" |
| No release signing | `.goreleaser.yaml`, `.github/workflows/*.yml` | `cosign`, `minisign`, `gpg`, `sign` | No signing step |
| No CLI JSON output | `internal/**/*.go`, `cmd/**/*.go` | `--json`, `"json"` | Only updater state JSON; no user-facing flag |
| No telemetry/analytics | `**/*.{go,md,yml,yaml,sh}` excluding `goals/` | `telemetry`, `analytics`, `sentry` | No SDK; governance anti-goal; "diff sentry" is a script name |
| No monetization | `README.md`, `GOVERNANCE.md`, `CONTRIBUTING.md`, `SECURITY.md`, `Makefile`, `.github/**` | `pricing`, `sponsor`, `donat`, `paid`, `subscription`, `commercial license`, `support contract`, `SLA` | No matches |
| No other agent runtimes | `internal/**/*.go`, `cmd/**/*.go` | `claude`, `codex`, `cursor`, `windsurf`, `cline`, `aider`, `opencode`, `gemini`, `copilot` | Only `codex-daemon-` in the excluded-daemon prefix list; no detection logic |
| No `/var/tmp` scan | `internal/cache/cache.go` | `ScanTmpDirectories`, `/tmp`, `/var/tmp` | Scanner hardcodes `/tmp`; cleaner allows `/var/tmp` |
| No tests for upgrade asset naming | `internal/updater/updater_test.go` | `assetName`, `arm`, `armv7`, release download | Tests cover checksum parsing and size limits, not asset-name contract |
| No benchmark suite | `**/*_test.go` | `Benchmark`, `b.N` | No benchmarks; performance claim is a single local run |
