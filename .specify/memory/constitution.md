<!--
Sync Impact Report - Constitution Update
==========================================
Version change: 1.0.0 → 2.0.0
Bump rationale: MAJOR — all five principles redefined; Technology Stack section replaced with
Development Standards; testing philosophy shifted from coverage targets to behavior-first.

Modified principles:
  - I. Simplicity and Maintainability → I. Simplicity Is Non-Negotiable (tightened, YAGNI made explicit)
  - II. Go Language → II. Idiomatic Go (renamed, stdlib-first rule added)
  - III. Latest and Robust Dependencies → IV. Minimal Dependencies (reordered, "earn its place" framing)
  - IV. Comprehensive Unit Testing → V. Pragmatic Testing (philosophy changed: behavior > coverage targets)
  - V. Code Quality and Taste → dissolved into Principles I, II, and Development Standards

Added sections:
  - Development Standards (replaces Technology Stack; absorbs formatting, naming, comment rules)

Removed sections:
  - Technology Stack (content folded into principles and Development Standards)

Templates requiring updates:
  ✅ .specify/templates/plan-template.md — "Constitution Check" section compatible; no update needed
  ✅ .specify/templates/spec-template.md — compatible; no update needed
  ✅ .specify/templates/tasks-template.md — compatible; no update needed

Follow-up TODOs: None — all placeholders resolved.
-->

# Polyoracle Constitution

## Core Principles

### I. Simplicity Is Non-Negotiable

The simplest correct solution MUST always be chosen. Complexity requires explicit written justification.

**Rules:**
- Solve the problem in front of you. Do not solve the problem you imagine coming next.
- Three similar lines of code beat a premature abstraction. Extract only when duplication causes bugs.
- If you cannot explain why an abstraction exists in one sentence, delete it.
- Functions MUST do one thing. If naming is hard, the scope is wrong.
- No design patterns for their own sake. No frameworks for problems that do not yet exist.

**Rationale:** Every layer of abstraction is a debt. Abstractions that do not pay off compound into
systems no one can change safely.

### II. Idiomatic Go

All code MUST be idiomatic Go. The standard library is the first and preferred solution.

**Rules:**
- Reach for `stdlib` before any third-party package. A dependency needs a written justification.
- Follow Go proverbs: errors are values, accept interfaces return structs, make the zero value useful.
- Concurrency MUST be explicit — goroutines and channels, not hidden behind opaque abstractions.
- All code MUST pass `go vet` and `gofmt` unmodified before commit.
- Generics are permitted only when the concrete, measurable benefit is obvious.

**Rationale:** Idiomatic Go is the language's core strength. Fighting its idioms creates friction;
working with them produces clarity.

### III. Explicit Error Handling

Errors MUST be handled explicitly at every call boundary.

**Rules:**
- Never discard an error with `_` unless a comment explains precisely why it is safe.
- Wrap errors with context (`fmt.Errorf("doing X: %w", err)`); never swallow them silently.
- Panics are reserved for programmer-error invariant violations, never for user or external system errors.
- No error type hierarchies unless the caller genuinely needs to distinguish between error cases.

**Rationale:** Silent errors become production mysteries. Explicit handling makes every failure mode
visible and debuggable without a debugger.

### IV. Minimal Dependencies

Each external dependency MUST earn its place.

**Rules:**
- Before adding a dependency, verify `stdlib` cannot solve the problem adequately.
- Dependencies MUST be pinned in `go.mod` and come from actively maintained projects.
- Prefer single-purpose, narrowly scoped libraries over large frameworks.
- The reason for each non-stdlib dependency MUST be documented near its usage or in package docs.

**Rationale:** Every dependency is a liability — build time, security surface, upgrade burden. Fewer
dependencies mean a system easier to audit, update, and understand.

### V. Pragmatic Testing

Test critical behavior, not implementation details.

**Rules:**
- Use table-driven tests for any logic with multiple input/output pairs.
- Tests MUST verify behavior (what the code does), not implementation (how it does it).
- Mock only external I/O boundaries: HTTP clients, filesystem, time, external APIs.
  Never mock internal collaborators.
- A test harder to maintain than the code it covers is a bad test — simplify or delete it.
- No coverage percentage targets. Cover what can realistically break in production.

**Rationale:** Tests that verify behavior survive refactoring. Tests that verify implementation
actively block it.

## Development Standards

**Formatting:** `gofmt` is mandatory. No custom formatters, no exceptions.

**Linting:** `golangci-lint` with the project config. Fix warnings — do not suppress them without
a comment explaining why the suppression is correct.

**Naming:** Clear, complete names over short ones. `userID` not `uid`. Acronyms follow Go convention
(`URL`, `ID`, `HTTP`). If naming is hard, the abstraction is wrong.

**Comments:** Comments explain *why*, not *what*. If code needs a comment to explain what it does,
rewrite the code until it is self-evident.

**Dead code:** Remove it. Commented-out code MUST NOT be committed. Version control is the history.

## Governance

Constitution compliance is verified at code review. Violations require explicit written justification
committed alongside the code, not merely verbal approval.

**Amendment procedure:** Increment the version, document the rationale, update dependent templates
if affected. No unanimous consensus required — one maintainer with clear reasoning suffices.

**Versioning policy:**
- MAJOR: Principle removal or incompatible redefinition of an existing principle.
- MINOR: New principle added or existing principle materially expanded.
- PATCH: Wording clarifications, typo fixes, non-semantic refinements.

**Compliance review:** All PRs MUST explicitly verify that no principle is violated. Exceptions are
temporary and MUST be tracked as TODOs with a deadline or linked issue.

**Version**: 2.0.0 | **Ratified**: 2026-02-16 | **Last Amended**: 2026-02-17
