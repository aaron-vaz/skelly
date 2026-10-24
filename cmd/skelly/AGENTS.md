# AGENTS.md (cmd/skelly)

End-to-end tests of the compiled `skelly` binary. Read this before adding or changing a test here. Everything else in the repo is covered by `/AGENTS.md`.

## How the tests run

`TestMain` in `main_test.go` builds the binary once (`skelly_test_bin`, removed afterwards) and each test runs it as a sub-process. Run them with:

```bash
go test -race -count=1 ./cmd/...
go test -race -count=1 -run TestIntegration_Inspect ./cmd/...   # one group
```

Use `-count=1`: the tests depend on the binary and the filesystem, so cached results can mislead.

## Where things live

| File | Covers |
|---|---|
| `main_test.go` | help, version, `init` basics (prompts, default destination, closed stdin, Ctrl+C, nested names, raw files), `sources` |
| `autocomplete_integration_test.go` | `autocomplete <shell>` scripts and the completions they give in real bash, zsh and fish (a shell that is not installed is skipped) |
| `install_integration_test.go` | `install.sh` against a fake releases server: checksum and provenance checks, bad versions and URLs, unsupported platforms, replacing a binary, a truncated script |
| `features_integration_test.go` | `exclude` and `choices`, `rename`, templated config values (derived inputs asked in dependency order, `env`, patterns, conditions), invalid configs, `inspect`, `-set` / `-no-input` / `-yes` / `-json` |

Put a test for a new command or flag in the file for its area, or in a new `<area>_integration_test.go`. Do not grow `main_test.go`.

## Helpers

- `writeTemplate(t, files)` creates a template directory from a `path -> content` map and returns it.
- `runSkelly(t, stdin, env, args...)` runs the binary with a **private `SKELLY_CONFIG`**, a 30 second timeout and separate `stdout` / `stderr`. Prefer it over `exec.Command`.
- `assertExists`, `assertAbsent`, `assertContent` check the generated project.
- `variantFiles()` / `variantConfig` is a template that exercises most features at once; reuse it before writing a new fixture. For templated values, the fixtures inside `TestIntegration_TemplatedConfig` and `TestIntegration_Rename` show how to build one.

## Conventions

- Follow the BDD comments from `/AGENTS.md` (`// Given`, `// When`, `// Then`, `// And`).
- Never touch the real config or home directory: pass `SKELLY_CONFIG` (done by `runSkelly`) and set `HOME` to a temp dir when a test could reach it.
- **stdout is for programs, stderr is for people.** Assert that `inspect` and `init -json` print only JSON on stdout, and that failures leave stdout empty with a non-zero exit code.
- Prompts are asked in alphabetical order of the input names, except that an input whose default, description or choices use other inputs is asked after them (in a later layer), so supply stdin in that order.
- A test that can block (prompts, signals) must have a timeout, as `runSkelly` does, so a regression fails instead of hanging CI.
- On failure paths also assert that the destination was **not** created or modified: `init` must only touch it after download and rendering succeeded.
- When a feature changes the shape of `.skelly.yml`, update `/skelly.schema.json` too; its unit test is in `internal/templates/schema_test.go`.
