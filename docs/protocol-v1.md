# Proof-preserving optimizer protocol v1

The protocol has four observable layers:

1. The `.gooo` source is parsed into a typed IR. The source declares the
   grammar, type/effect/capability rules, origin map, rewrite rules, proof
   obligations, fixed denominator, precedence, and cost vector.
2. A declared rewrite produces a new IR plus a rewrite record. The only
   admitted rewrites are pure constant folding, effect-free dead branch
   elimination, and deterministic first-left-to-right pure CSE.
3. Baseline and optimized IRs each lower to a standalone generated Go program.
   The same CI runner builds and executes both programs with Go 1.27. Their
   output includes the value, terminal reason, effect/capability summaries,
   ordered traces, and provenance anchors.
4. The verifier compares the complete observable tuple and records exact
   before/after cost evidence. A missing cost witness is UNKNOWN and a semantic
   mismatch is REFUTED.

## Admitted proof obligation

For a candidate pair `(B, O)`, closure requires all of the following:

```text
type(B) = type(O)
effect(B) = effect(O)
capability(B) = capability(O)
terminal_reason(B) = terminal_reason(O)
ordered_effect_trace(B) = ordered_effect_trace(O)
capability_trace(B) = capability_trace(O)
source_origin_anchors(B) = source_origin_anchors(O)
generated_output(B) = generated_output(O)
```

The branch-elimination rule adds a syntactic guard: the discarded subtree must
be `PURE` with `NONE` capability. This is deliberately conservative even when
a particular constant condition would make an effect unreachable. It prevents
the optimizer from silently widening its proof boundary.

## Exact cost pair

Each closed case has two separate vectors:

```json
{
  "binary_bytes": 0,
  "generated_bytes": 0,
  "wall_ms": 0,
  "peak_rss_kib": 0
}
```

The pair identity binds `scenario_id`, source digest, contract digest, Go
toolchain digest, and runner digest. No aggregate metric is used to close a
case or call an optimization an improvement.

## State precedence

The protocol is fail-closed and resolves evidence as
`REFUTED > UNKNOWN > CLOSED`. Every UNKNOWN record carries the six fields
declared by the `.gooo` source: `stage`, `step`, `reason`, `unknown_class`,
`next_operation`, and `blocked_by`.
