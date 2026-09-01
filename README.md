# Gooo Proof-Preserving IR Optimizer

This repository is a deliberately narrow optimizer laboratory. Its fixed
eight-case vector proves that three real IR rewrites can reduce generated Go
while preserving the declared type, capability, effect, terminal reason,
ordered effect trace, and provenance anchors:

| Case | Expected | Boundary exercised |
| --- | --- | --- |
| `operator-constant-fold` | `CLOSED` | pure constant folding |
| `operator-dead-branch` | `CLOSED` | effect-free dead branch elimination |
| `operator-deterministic-cse` | `CLOSED` | deterministic left-to-right CSE |
| `comment-only` | `CLOSED` | comment/whitespace normalization |
| `effectful-branch-removal` | `REFUTED` | discarded IO effect |
| `origin-loss` | `REFUTED` | missing provenance anchors |
| `missing-cost-witness` | `UNKNOWN` | absent exact cost evidence |
| `replay` | `CLOSED` | deterministic generation and replay |

The authority is [`.gooo/proof-preserving-ir-optimizer.gooo`](.gooo/proof-preserving-ir-optimizer.gooo).
It declares the grammar, typed IR, rewrite rules, effect and capability
lattices, forbidden effects, origin map, proof obligations, fixed denominator,
resolution precedence, and exact cost observation policy. Go parses and lowers
those declarations, then generates and executes caller-owned temporary Go
artifacts and independently verifies the observations.

Every case records exact integer `binary_bytes`, `generated_bytes`, `wall_ms`,
and `peak_rss_kib` before and after. A metric is called an improvement only when
the semantic predicates close first and the before/after pair has the same
scenario, source, contract, Go toolchain, and runner digest. No aggregate score,
weighted average, percentage, or estimate is emitted.

The conformance command is CI-only by project contract:

```text
gooo-proof-preserving-ir-optimizer conformance \
  --meta .gooo/proof-preserving-ir-optimizer.gooo \
  --root . --out "$RUNNER_TEMP/gooo-proof-preserving-ir-optimizer"
```

All generated files and binaries are placed below the caller-owned `--out`
directory. Repository writes, local test/build/vet/conformance/integration
executions, automatic commits, pushes, merges, and releases are zero.

## Proof boundary

CompCert's documented compiler theorem is a useful reference point: each pass
is justified by a semantic-preservation proof and the observable behavior of
generated code is related back to the source behavior. This project does not
claim a whole-compiler theorem. Its falsifiable boundary is smaller and
explicit: the three declared rewrites are admitted only for this typed IR and
only when the generated baseline and optimized programs agree on the declared
observable tuple and every source-origin anchor. Any discarded IO,
capability-bearing, or nondeterministic subtree is rejected; missing cost
evidence is UNKNOWN.

LLVM's language reference also motivates the conservative boundary: apparently
simple branch or value rewrites can change behavior in the presence of poison,
undefined behavior, or observable effects. Gooo therefore has no implicit
undefined-value or speculative-execution rule; an effect not declared PURE is
never silently discarded.

Primary references: [CompCert semantic preservation](https://compcert.org/man/manual001.html),
[CompCert constant-propagation proof](https://compcert.org/doc/html/compcert.backend.Constpropproof.html),
and the [LLVM Language Reference](https://llvm.org/docs/LangRef.html).

## Release boundary

The release workflow is manually dispatched only on `main` with an exact merge
SHA and a never-reused `0.x.y` version. It refuses existing tags/releases,
creates an annotated tag, uploads digest-bound draft assets, publishes once,
and checks the public release API for `immutable=true` plus every asset digest.
It never calls a repository admin-settings endpoint with `GITHUB_TOKEN` and has
no tag/release overwrite or delete path. The immutable-releases setting must be
enabled through the user API before a release is considered closed.
