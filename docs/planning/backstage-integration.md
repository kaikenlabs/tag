# Backstage integration — TAG JSON contract plan

Verified against HEAD `9c7acf9` with a binary built from source. The checked-in `./tag` is stale — do not
verify against it. **Rev 2:** independently reviewed by Codex (`gpt-5.6-sol`) and Antigravity (`--tier pro`);
every peer claim was re-verified against source before being folded in. Peer-sourced findings are marked
⁽ᶜ⁾ Codex / ⁽ᵃ⁾ Antigravity.

## 0. Decisions taken

All ten open questions are resolved; nothing below blocks implementation.

| Question | Decision | Consequence |
|---|---|---|
| Ref allowlist | **Strict enumerated list** in Backstage config; the action rejects any ref not literally listed | Closes the SSRF / local-path surface (`remote/reference.go:63-65,99,108,184`) at the action layer. **No TAG change.** Every catalog template is platform-vetted, which also bounds the hook risk. |
| Cache isolation | **Per-tenant cache directory** via `TAG_CACHE_DIR` | Mitigates C6 without touching `CacheKey()`. **Makes item 1 a launch blocker** — no cache-dir override exists today. C8 stays latent but scoped within a tenant. |
| Tree root | **Add `project_root`, keep `output_dir`** | Additive, ~1h; `ScaffoldResult.ProjectRoot` already holds it (`types.go:61`). No consumer breaks. Publish `project_root`. |
| Error stream | **JSON document on stdout, human line on stderr**, non-zero exit | Action parses stdout; the Backstage task log stays readable. Requires the `cmdOut` write-guard so the four conflict commands do not emit twice. `slog` is *reformatted*, not silenced. |
| v1 scope | **`template info` + `scaffold` only** | The other 20 commands keep today's behaviour. The inconsistency is deliberate and documented, not accidental. |
| Variable field name | **Keep `prompted`** | Matches scaffold's internal vocabulary; centralises the `!private && !derived` rule so consumers cannot drift from it. |
| Statelessness | **Add `--no-library`; reuse the existing `--no-save`** | ~3h, not ~1 day. No umbrella flag to maintain. Caveat: disabling `addToLib` re-enables generator copying into the project (`scaffold.go:311-313`) — correct for a standalone repo, but a visible difference. |
| SSH fallback | **Deployment-only — no agent, no `~/.ssh` keys in the image** | Zero code; verified sufficient (`auth.go:89-99` falls through to HTTPS). Residual: a convention nothing enforces, so a base-image change can silently reopen it. |
| Template validation | **Entity provider calls `tag template lint --format json` separately** | Zero TAG change; `lint.Result` (`internal/lint/types.go:12-24`) already carries the diagnostics `info` lacks. |
| Distribution | **`install.sh` in the backend image; npm dropped from scope** | Saves 3–5 days and avoids the `tag upgrade` self-replace conflict. npm remains a separate goal for other Node users, unblocked by this work. |

## 1. Findings

| # | Claim | Verdict | Evidence |
|---|---|---|---|
| A1 | `prompt` absent from JSON | **Fails** | Emitted at `info.go:313`, populated `:346`; verified. The docs example (`template.md:196-206`) just uses vars with no prompt, and `omitempty` hides it. The **text** view drops it (`info.go:198-210`) — drift runs the other way. |
| A1b | One struct could feed both renderers | **Caveat** ⁽ᶜ⁾ | `info.go:290-298` forbids marshalling the *domain* struct; it does not forbid both renderers consuming the resolved `templateInfoJSON`. A judgement call, not a reproducible defect. Still keep them separate — the text view is golden-pinned — but rev 1 overstated it as impossible. |
| A2 | Derived/private indistinguishable | **Holds** | `buildTemplateInfoJSON` (`info.go:334-352`) iterates all vars unfiltered; `_internal` and `derived_slug` appear unmarked. |
| A2b | A reusable predicate exists | **Holds** | `IsPrivate`/`IsDerived`/`IsEvaluatedDefault` (`tmplconfig/types.go:48,60,80`). Rule is `!IsPrivate && !IsDerived`, already used at `scaffold/variables.go:176-183`, `scaffold.go:193`, `vars/types.go:81-82`. |
| A3 | Defaults arrive as raw expressions | **Holds** | Verified: `"default":"…/{{ vars.project_name }}"`. |
| A3b | Resolver already marks these | **Holds** | `IsEvaluatedDefault()` is exactly this flag. `template variables` already ships `derived`/`private` (`vars/types.go:19-20`) but not `prompt`/`secret`. |
| A4 | No `schema_version`/`tag_version` | **Holds** | None. `jsonout.go:12-20` rejects an *envelope*; a root-level key on a bare document is a different thing (see §2 item 4). |
| B1 | Each command builds JSON ad hoc | **Caveat** | 22 commands (`format_conformance_test.go:158-181`). `jsonout.Write` fixes *encoding* only: 16 named DTOs, 4 inline `map[string]any`, and **`test` + `generate info` bypass `jsonout`** (`testrunner/report.go:83-87`, `generate_info.go:103-107`). |
| B2 | Error output undocumented | **Holds — worst gap** | For `scaffold`/`info`, stdout is **zero bytes** and stderr gets a timestamped slog line (`main.go:79`). **Not universal** ⁽ᶜ⁾: `check`, `generate`, `undo`, `update` write a success-shaped document *then* exit non-zero (`generate.go:256-268`, `check.go:72-81`, `undo.go:232`, `update_template.go:285`). Any universal handler must not append a second document. |
| B2b | Exit codes | **Caveat** | Table exists (`pkg/app/errors.go:8-14`: 0/1/2/3/130) but `ExitNotFound=3` is used at 4 sites only — not-found, auth-failure and bad-path **all return 1**. |
| B2c | Existing types support a code | **Holds** | 12 sentinels + 18 typed errors. `ErrAuthRequired` **already exists**, with `AuthError.Is` mapping onto it ⁽ᶜ⁾ (`remote/errors.go:14`, `:101`) — what is missing is a *serialized* code, not the sentinel. Some sites flatten with `%s`, severing the chain (`library.go:301-307`). |
| C1 | Cache can corrupt; locking? | **Fixed by #397** — `FSCache.Set` now builds each entry in a `.staging-*` dir and publishes via rename; replay/library/history/conflict-status writers all use unique temp names + `fileutil.WriteFileAtomic`. Originally: **Holds — no locking at all**. `FSCache.Set` did `RemoveAll` → `MkdirAll` → `CopyDir` **in place** (`remote/cache.go:129-139`). Zero locks in `internal/remote`; floating refs always re-`Set` (`remote.go:77`). Correction ⁽ᶜ⁾: replay/library are **not** a safe counter-example — both use a fixed `<file>.tmp` (`replay/save.go:64`, `library/registry.go:75`), so same-ref saves race; `registry.go:29-32` documents itself process-unsafe. |
| C2 | Paths overridable | **Fails for the ones that matter** | `--path`/`--shared-path`/`--bundle-path` have flags + `TAG_*` env (`flags.go:247-264`). **Cache and replay have neither and ignore XDG** (`remote/cache.go:16,58`; `replay/save.go:100-105`). |
| C3 | `--format json` implies `--no-save` | **Fails** | `jsonMode` feeds `NoInput` only (`flags.go:345`); `NoSave` reads its flag (`:348`). A `--values --format json` run wrote `~/.tag/replay/`. Secrets *are* stripped. |
| C4 | TTY/`$HOME` at startup | **Caveat** | Startup is clean (`main.go:37-48`); TTY probes lazy and fail safe. But `NewResolver()` **eagerly `MkdirAll`s `~/.tag/cache`** (`remote/cache.go:66`) before knowing the ref is local. |
| **C5** | **`--no-save` does not make a run stateless** | **New** ⁽ᶜ⁾ | `addToLib := isRemote` (`scaffold.go:307`) — every remote scaffold mutates the shared XDG library and sets `SkipGeneratorCopy=true`. `--no-save` governs replay only (`scaffold.go:502`). Library names derive from the repo basename (`remote/reference.go:529`), so two orgs sharing a repo name collide silently. |
| **C6** | **Cache crosses credential boundaries** | **New — security** ⁽ᶜ⁾ | `CacheKey()` = `{provider}_{owner}_{repo}[@version]`, no tenant or credential identity (`remote/reference.go:355-369`). The cache is consulted at `remote.go:79-81`, **before** the fetcher and therefore before any auth. A private template fetched with tenant A's token is served to tenant B on a pinned-ref hit. |
| **C7** | **SSH agent fallback defeats per-call credentials** | **New — security** ⁽ᶜ⁾ | An HTTPS ref with no token tries the process's SSH agent/keys **first** (`remote/git.go:150-155`). A pod with a mounted deploy key uses it for every tenant. |
| **C8** | **Subpath caching is structurally broken** | **New** ⁽ᶜ⁾ | `CacheKey` omits `SubPath` (`reference.go:355`) while the fetcher returns `tmp/<subpath>` (`git.go:76`) — monorepo templates overwrite one entry instead of getting hits. |
| D1/D2 | `output_dir` names the tree root | **Fails — highest-value finding** | Wrapper template + explicit `--output` does **not** unwrap (`scaffold.go:281-290`): files land in `<output>/<project_name>/` while `output_dir` reports the parent. `ScaffoldResult.ProjectRoot` already holds the right value (`types.go:61`) and is simply not emitted. |
| D3 | `files[].path` bases differ | **Fails** | Same base (`output_dir`) both cases — `output.go:202`. The doc's `my-project/main.go` is stale: text dry-run prints the **absolute** path (`output.go:247,370`). `files[]` omits `.tagconfig.json`/`.tag/history.json`, so `created` undercounts. |
| D4 | Tokens per-process; proxy | **Holds** | `os.LookupEnv` per call (`remote/auth.go:41`). Proxy honoured both paths (`zip.go:33`; go-git uses `http.DefaultTransport`). No custom-CA knob. |
| D5 | Hook skipping enforced for remote | **Fails** | `ConfirmHooks` (`hooks/hooks.go:323-352`) is **source-agnostic** — the gate is interactivity, not remoteness. `scaffold.md:45,350-363` is wrong both ways. |
| **D6** | **`info` can advertise a template `scaffold` rejects** | **New** ⁽ᶜ⁾ | `info` only parses (`info.go:148-168`); `scaffold` also schema-validates (`scaffold.go:545`) and circular deps surface only in the collector (`variables.go:518`). `info` also discards `CommitSHA` (`info.go:129`), omits `keywords`/`categories` (`tmplconfig/types.go:14,16`), and sorts alphabetically where scaffold prompts topologically (`variables.go:45`). |
| E1 | Release matrix | **Holds** | 5 binary targets, `CGO_ENABLED=0` (`.goreleaser.yml:12-23`), checksums, **no signing**, no publishers. Auto-fires on every green CI on `main` (`release.yml:3-19`). |
| E2 | No postinstall fetch needed | **Holds** | All assets `go:embed` (`schema/embed.go:7`, `dialect/embed.go:5`, `main.go:28-35`); no network at startup. `tag upgrade` self-replaces the binary, which would fight npm. |

## 2. Proposed changes (dependency order)

Total ≈ **4 days**. npm packaging is out of scope per §0.

| # | Change | Effort | Breaks consumers? | Non-Backstage benefit |
|---|---|---|---|---|
| 1 | **`TAG_CACHE_DIR`/`TAG_REPLAY_DIR` override + lazy `MkdirAll` — LAUNCH BLOCKER** (§0) | **~4h** | No | Containers, CI, sandboxed `$HOME` |
| 2 | Emit `project_root` in `scaffoldDoc` | **~1h** | No (additive) | Anyone scripting scaffold; fixes the stale text summary (`scaffold.go:588,595`) |
| 3 | Add `prompted`/`derived`/`private`/`default_is_expression` to `templateInfoVariableJSON` via the existing predicates | **~3h** | No (additive) | Any form/UI generator |
| 4 | `schema_version` + `tag_version` on the `info` and `scaffold` DTOs | **~2h** | No (additive) | Any pinned consumer |
| 5 | Add `resolved_commit`, `keywords`, `categories`, `depends_on` to `info` | **~4h** | No (additive) | Auditability; catalog metadata |
| 6 | Add `--no-library` (pairs with existing `--no-save`) | **~3h** | No (opt-in) | CI, sandboxes, anyone who does not want a scaffold mutating global state |
| 7 | Print `prompt` in the **text** view of `template info` | **~1h** | Golden fixture changes — deliberate | Every CLI user: labels are currently invisible |
| 8 | JSON error document **on stdout**, human line retained on stderr | **~0.5–1 day** (info+scaffold) | Yes — stdout is no longer empty on failure | Every scripted consumer |
| 9 | ~~Atomic cache `Set` (unique temp + `os.Rename`); fix the fixed-`.tmp` races in replay/library~~ **Done (#397)** | **~6h** | No | Anyone running two `tag` processes at once |

**Item 1 first** because §0's cache isolation cannot be deployed without it.

**Item 8.** Rev 1 proposed stderr plus a package-level `jsonMode` var; both peers rejected stderr and Codex
rightly called the global var poor design. `ExitErrHandler` receives the **command's** context (verified,
`urfave/cli@v2.27.7/command.go:276-278`), so format resolves at the error seam with no global state. The
hazard is the four commands that already write a document and exit non-zero (row B2) — guard by tracking
"has stdout been written" at the `cmdOut` chokepoint (`flags.go:70-75`). Contract: **exactly one JSON
document on stdout in JSON mode, success or error, exit code decides.** Per §0 the stderr line is kept but
should be plain (drop the `[HH:MM:SS.mmm]` prefix and ANSI colour) so the task log reads cleanly.

**Rejected:** stamping `schema_version` on all 22 documents. Item 4 puts it on the two surfaces Backstage
consumes — both peers preferred this over rev 1's "route it to `tag version`". **Rev 1 error:** the
exit-code item read "add `ErrAuthRequired`"; it already exists, only a serialized code is missing.
Exit-code renumbering is deferred in favour of `error.code` ⁽ᶜ⁾.

## 3. Recommended v1 contract

`tag template info --format json`:

```json
{
  "schema_version": 1,
  "tag_version": "v1.4.2",
  "name": "go-api", "description": "Go REST API", "version": "v1.2.0",
  "template": "gh:org/repo@v1.2.0",
  "resolved_commit": "a1b2c3d4",
  "keywords": ["go", "rest"], "categories": ["backend"],
  "variables": [
    { "name": "project_name", "type": "string", "prompt": "Project name",
      "default": "my-proj", "required": false, "secret": false,
      "prompted": true, "derived": false, "private": false,
      "default_is_expression": false, "depends_on": [] },
    { "name": "module_path", "type": "string", "prompt": "Go module path",
      "default": "bitbucket.org/myorg/{{ vars.project_name }}",
      "required": false, "secret": false,
      "prompted": true, "derived": false, "private": false,
      "default_is_expression": true, "depends_on": ["project_name"] },
    { "name": "derived_slug", "type": "string", "default": "{{ vars.project_name }}-svc",
      "required": false, "secret": false,
      "prompted": false, "derived": true, "private": false,
      "default_is_expression": true, "depends_on": ["project_name"] }
  ],
  "hooks": { "pre_scaffold": [], "post_scaffold": ["go mod tidy"] },
  "has_readme": true, "has_howto": false
}
```

`prompted` is `!private && !derived` — the only question a UI asks; `derived`/`private` explain it.
`default_is_expression` says don't render `default` literally; `depends_on` names what it needs. All
always present (no `omitempty`). **A `secret` default must never become a Backstage schema default** ⁽ᶜ⁾.

`tag scaffold --format json`:

```json
{
  "schema_version": 1, "tag_version": "v1.4.2",
  "output_dir": "/ws",
  "project_root": "/ws/my-proj",
  "template": "gh:org/repo@v1.2.0",
  "resolved_commit": "a1b2c3d4",
  "files": [ { "path": "my-proj/README.md", "action": "create" } ],
  "created": 1, "dry_run": false
}
```

`project_root` equals `output_dir` for non-wrapper templates. `files[].path` stays relative to
`output_dir`, and `files`/`created` exclude `.tagconfig.json` and `.tag/history.json`.
**Publish `project_root`, not `output_dir`.**

Error document — **stdout**, non-zero exit, exactly one document either way. A plain human line goes to
stderr in parallel for the Backstage task log (§0):

```json
{ "schema_version": 1, "tag_version": "v1.4.2",
  "error": { "code": "template_not_found", "message": "…", "exit_code": 3 } }
```

Initial codes from existing sentinels: `template_not_found`, `auth_required`, `version_not_found`,
`required_variable_missing`, `output_exists`, `circular_dependency`, `usage`, `internal`.

## 4. Deferred

| Item | Trigger |
|---|---|
| Cache locking (flock) | Item 6 proves insufficient under real load |
| Credential-aware cache keys | Per-tenant directories (§0) prove insufficient — e.g. one tenant serving several credential scopes |
| TAG-side `--allow-ref` enforcement | A second, non-Backstage caller appears, or the action-layer allowlist is bypassed |
| npm packaging (`@kaikenlabs/tag-cli`) | Node users outside this integration ask for it — 3–5 days, six packages, and needs the `tag upgrade` conflict resolved |
| `TAG_NO_SSH_FALLBACK` flag | Image contents can no longer be guaranteed, making §0's deployment convention insufficient |
| `--stateless` umbrella flag | A third piece of global state appears and two flags stop being enough |
| Validation status inside `template info` | The provider's second `lint` spawn becomes a measured problem |
| Subpath in `CacheKey` | First monorepo template in the catalog |
| Exit-code renumbering (1→3) | A major release; `error.code` covers v1 |
| Per-template hook allowlist | Any template you actually want hooks for |
| Error document across all 22 commands | A consumer needs something outside `info`/`scaffold` |
| Binary signing / SBOM | First security review of the npm supply chain |
| Unifying `test`/`generate info` onto `jsonout` | A third bypass appears |

## 5. Open questions

None outstanding — all ten are recorded as decisions in §0. Items that would reopen a decision are listed
in §4 with their triggers.

## 6. Risks

- **Cross-tenant template disclosure (C6).** *Mitigated by §0* — per-tenant `TAG_CACHE_DIR`. Residual: the
  mitigation is deployment configuration, not a code guard, so a missing/misset `TAG_CACHE_DIR` silently
  restores the shared cache and the leak. Fail the action at startup if it is unset.
- **Ambient SSH credentials (C7).** *Mitigated by §0* — no agent or keys in the image. Residual: this is a
  deployment convention with no code guard, so a base-image change can silently reopen it. Consider a
  startup assertion that `SSH_AUTH_SOCK` is unset and `~/.ssh` is absent.
- **Every remote scaffold mutates shared global state (C5)**, `--no-save` notwithstanding. *Addressed by
  item 6* (`--no-library`); until it ships, library names collide on repo basename.
- **`--force` deletes the whole `--output` directory** (`scaffold.go:493`) — verified. Scaffold into an
  empty subdirectory. Its safety check runs **only under `--force`** ⁽ᶜ⁾, so the ≥2-segment / not-`/tmp`
  restriction is not a general `--output` rule.
- **Cache corruption between concurrent processes** — no locks, destructive in-place write. Scoped ⁽ᶜ⁾:
  needs a *shared* cache directory; replicas on separate filesystems cannot race each other.
- **Hooks inherit the whole process environment** (`hookenv.go:36`), so an accepted hook from a remote
  template exfiltrates `GITHUB_TOKEN`. Never pass `--accept-hooks` in multi-tenant use.
- **Arbitrary refs are an SSRF and resource-exhaustion surface** (`remote/reference.go:98`) ⁽ᶜ⁾.
  *Mitigated by §0* — strict enumerated allowlist. Residual: enforcement lives entirely in the TypeScript
  action; TAG itself still accepts any ref, so a second caller or a bug reopens it.
- **Every green merge to `main` cuts a release** (`release.yml:3-19`), unsigned. Pin the version in the
  image build; `install.sh --version` does this and verifies sha256.
- **`created` undercounts** written files; not a completeness check.
- **Docs are unreliable in both directions**: the hook/remote claim, the dry-run path example, and the
  `template info` text example (`Source:`/`Path:`/`Generators:` are never printed — `info.go:171-181`) are
  all wrong. Verify against a built binary before encoding anything in the plugin.
