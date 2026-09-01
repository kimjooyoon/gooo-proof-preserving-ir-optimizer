# Repository ownership

This repository is independently owned by the task that created
`gooo-proof-preserving-ir-optimizer`. Its scope is limited to this directory.

The `.gooo` contract is authoritative for grammar, typed IR, rewrite rules,
effects, capabilities, origin preservation, fixed denominator, precedence,
proof obligations, and cost observation. Go code may parse, lower, generate,
execute, and verify that contract; it may not silently add semantic rules.

Development authority is read-only with respect to the repository:
`repository_writes=0`, caller-owned temporary output only, and automatic
commit/push/merge/release authority are all zero. Build, test, vet,
conformance, and integration execution belongs to GitHub Actions.
