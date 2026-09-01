package optimizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func LoadContractFile(path string) (Contract, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, nil, err
	}
	contract, err := ParseContract(string(raw))
	if err != nil {
		return Contract{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	contract.SourceDigest = Digest(raw)
	contract.SourceCanonicalKey = Digest([]byte(ContractCanonical(contract)))
	return contract, raw, nil
}

func RunConformance(metaPath, repositoryRoot, outputRoot string) (ConformanceReport, error) {
	contract, rawMeta, err := LoadContractFile(metaPath)
	if err != nil {
		return ConformanceReport{}, err
	}
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return ConformanceReport{}, err
	}
	out, err := filepath.Abs(outputRoot)
	if err != nil {
		return ConformanceReport{}, err
	}
	if pathInside(root, out) {
		return ConformanceReport{}, fmt.Errorf("output must be caller-owned and outside the input repository")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return ConformanceReport{}, err
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		return ConformanceReport{}, fmt.Errorf("output directory must be empty or absent")
	}

	toolchainDigest, err := digestToolchain()
	if err != nil {
		return ConformanceReport{}, err
	}
	runnerDigest := digestRunner()
	if err := WriteJSON(filepath.Join(out, "contract.json"), contract); err != nil {
		return ConformanceReport{}, err
	}

	programs := map[string]ProgramDecl{}
	for _, program := range contract.Programs {
		programs[program.ID] = program
	}
	cases := make([]CaseReport, 0, len(contract.Scenarios))
	for _, scenario := range contract.Scenarios {
		program, ok := programs[scenario.Program]
		if !ok {
			return ConformanceReport{}, fmt.Errorf("scenario %q references unknown program %q", scenario.ID, scenario.Program)
		}
		caseReport, err := runCase(contract, rawMeta, scenario, program, out, toolchainDigest, runnerDigest)
		if err != nil {
			return ConformanceReport{}, fmt.Errorf("case %s: %w", scenario.ID, err)
		}
		cases = append(cases, caseReport)
	}

	report := ConformanceReport{
		Schema:             "gooo-proof-preserving-ir-optimizer/conformance/v1",
		Decision:           Closed,
		ObservedPrecedence: []Decision{Refuted, Unknown, Closed},
		FixedDenominator:   contract.DenominatorCount,
		SourceDigest:       contract.SourceDigest,
		ContractDigest:     contract.SourceDigest,
		ToolchainDigest:    toolchainDigest,
		RunnerDigest:       runnerDigest,
		Summary:            Summary{TotalCases: len(cases)},
		Authority:          contract.AuthorityRule,
		Cases:              cases,
		NoAggregateMetrics: true,
	}
	for _, item := range cases {
		switch item.Observed {
		case Closed:
			report.Summary.Closed++
		case Unknown:
			report.Summary.Unknown++
		case Refuted:
			report.Summary.Refuted++
		}
		report.Generated.GeneratedGoFiles += 2
		report.Generated.GeneratedBinaries += 2
		report.Generated.GeneratedGoBytes += item.Before.GeneratedBytes + item.After.GeneratedBytes
		report.Generated.BinaryBytes += item.Before.BinaryBytes + item.After.BinaryBytes
		if item.Observed != item.Expected {
			report.Decision = Refuted
		}
	}
	if report.Decision == Refuted {
		return report, fmt.Errorf("fixed conformance vector did not match declared expectations")
	}
	if err := WriteJSON(filepath.Join(out, "conformance.json"), report); err != nil {
		return ConformanceReport{}, err
	}
	if err := os.WriteFile(filepath.Join(out, "conformance.md"), []byte(RenderMarkdown(report)), 0o644); err != nil {
		return ConformanceReport{}, err
	}
	return report, nil
}

func runCase(contract Contract, rawMeta []byte, scenario Scenario, program ProgramDecl, out, toolchainDigest, runnerDigest string) (CaseReport, error) {
	baseExpr := program.Expr.Clone()
	if scenario.Variant == "comment-only" {
		commented := commentOnlySource(string(rawMeta))
		commentContract, err := ParseContract(commented)
		if err != nil {
			return CaseReport{}, fmt.Errorf("comment-only source failed to parse: %w", err)
		}
		if ContractCanonical(commentContract) != ContractCanonical(contract) {
			return CaseReport{}, fmt.Errorf("comment-only source changed semantic contract")
		}
		for _, candidate := range commentContract.Programs {
			if candidate.ID == program.ID {
				baseExpr = candidate.Expr.Clone()
				break
			}
		}
	}
	if err := DeriveMetadata(baseExpr); err != nil {
		return CaseReport{}, err
	}
	optimized := baseExpr.Clone()
	var records []RewriteRecord
	var err error
	switch scenario.Variant {
	case "forbidden-effect":
		optimized, err = unsafeDeadBranch(baseExpr.Clone())
	case "operator", "comment-only", "origin-loss", "missing-cost", "replay":
		optimized, records, err = Optimize(baseExpr, scenario.Operator)
	default:
		return CaseReport{}, fmt.Errorf("unknown scenario variant %q", scenario.Variant)
	}
	if err != nil {
		return CaseReport{}, err
	}
	if scenario.Variant == "origin-loss" {
		optimized.Anchors = nil
	}
	if err := DeriveMetadata(optimized); err != nil {
		return CaseReport{}, err
	}

	caseDir := filepath.Join(out, scenario.ID)
	before, err := buildAndRun(caseDir, "before", baseExpr, generatedMetadataFor(baseExpr, nil))
	if err != nil {
		return CaseReport{}, fmt.Errorf("baseline generated program: %w", err)
	}
	var provenanceOverride []string
	if scenario.Variant == "origin-loss" {
		provenanceOverride = []string{}
	}
	after, err := buildAndRun(caseDir, "after", optimized, generatedMetadataFor(optimized, provenanceOverride))
	if err != nil {
		return CaseReport{}, fmt.Errorf("optimized generated program: %w", err)
	}

	baselineExpected, err := Evaluate(baseExpr)
	if err != nil {
		return CaseReport{}, err
	}
	if baselineExpected.Value != program.Result || baselineExpected.TerminalReason != program.Reason {
		return CaseReport{}, fmt.Errorf("program declaration does not match its independent interpreter")
	}
	proof := compareProof(baseExpr, optimized, baselineExpected, before.Output, after.Output)
	unknown := (*UnknownEvidence)(nil)
	refutations := append([]Refutation(nil), proofRefutations(proof)...)
	costWitness := scenario.Variant != "missing-cost"
	observed := proof.State
	if observed == Closed && !costWitness {
		observed = Unknown
		unknown = &UnknownEvidence{Stage: "COST", Step: "compare-exact-before-after", Reason: "EXACT_BEFORE_AFTER_COST_WITNESS_MISSING", UnknownClass: "MISSING_COST_WITNESS", NextOperation: "REMEASURE_BASELINE_AND_OPTIMIZED", BlockedBy: []string{"before_after_cost_vector"}}
	}
	if proof.State == Refuted {
		observed = Refuted
	}

	pairIdentity := PairIdentity{ScenarioID: scenario.ID, SourceDigest: contract.SourceDigest, ContractDigest: contract.SourceDigest, ToolchainDigest: toolchainDigest, RunnerDigest: runnerDigest}
	improvement := ImprovementClaim{State: observed, ExactPair: costWitness, Before: metricOf(before), After: metricOf(after)}
	if observed == Closed && costWitness && samePair(pairIdentity) && after.GeneratedBytes < before.GeneratedBytes {
		improvement.State = Closed
		improvement.Reason = "EXACT_PAIR_SEMANTICALLY_CLOSED_GENERATED_BYTES_REDUCED"
	} else if observed == Closed {
		improvement.Reason = "EXACT_PAIR_SEMANTICALLY_CLOSED_NO_REDUCTION"
	} else if observed == Unknown {
		improvement.Reason = "COST_WITNESS_REQUIRED_BEFORE_IMPROVEMENT"
	} else {
		improvement.Reason = proof.Reason
	}

	caseReport := CaseReport{ID: scenario.ID, Expected: scenario.Expected, Observed: observed, Operator: scenario.Operator, Variant: scenario.Variant, SourceDigest: contract.SourceDigest, SemanticSourceKey: contract.SourceCanonicalKey, ContractDigest: contract.SourceDigest, ToolchainDigest: toolchainDigest, RunnerDigest: runnerDigest, PairIdentity: pairIdentity, Before: before, After: after, Proof: proof, CostWitness: costWitness, Improvement: improvement, Unknown: unknown, Refutations: refutations, RewriteRecords: records}
	if scenario.Replay {
		replayDir := filepath.Join(caseDir, "replay")
		replayCode, replayErr := GenerateGo(optimized, generatedMetadataFor(optimized, provenanceOverride))
		if replayErr != nil {
			return CaseReport{}, replayErr
		}
		replayResult, replayErr := buildAndRun(replayDir, "replayed", optimized, generatedMetadataFor(optimized, provenanceOverride))
		if replayErr != nil {
			return CaseReport{}, replayErr
		}
		generatedSame := bytes.Equal(replayCode, readGenerated(filepath.Join(caseDir, "after", "generated", "main.go")))
		outputSame := outputEqual(after.Output, replayResult.Output)
		terminalSame := after.Output.TerminalReason == replayResult.Output.TerminalReason
		replayState := Closed
		if !generatedSame || !outputSame || !terminalSame {
			replayState = Refuted
			caseReport.Observed = Refuted
			caseReport.Refutations = append(caseReport.Refutations, Refutation{Kind: "REPLAY_MISMATCH", Reason: "REPLAY_GENERATED_OUTPUT_OR_TERMINAL_DIFFERED"})
		}
		caseReport.Replay = &ReplayEvidence{State: replayState, GeneratedSame: generatedSame, OutputSame: outputSame, TerminalSame: terminalSame, ReplayDigest: Digest(replayCode)}
	}
	return caseReport, nil
}

func generatedMetadataFor(expr *Expr, provenance []string) generatedMetadata {
	metadata := generatedMetadata{Effect: expr.Effect, Capability: expr.Capability, TerminalReason: terminalReasonFromExpr(expr)}
	if provenance != nil {
		metadata.ProvenanceAnchors = provenance
	}
	return metadata
}

func compareProof(beforeExpr, afterExpr *Expr, expected, before, after RuntimeResult) ProofReport {
	proof := ProofReport{
		State:                    Closed,
		TypePreserved:            beforeExpr.Type == afterExpr.Type,
		EffectPreserved:          beforeExpr.Effect == afterExpr.Effect,
		CapabilityPreserved:      beforeExpr.Capability == afterExpr.Capability,
		TerminalReasonPreserved:  before.TerminalReason == after.TerminalReason,
		EffectTracePreserved:     equalStrings(before.EffectTrace, after.EffectTrace),
		CapabilityTracePreserved: equalStrings(before.CapabilityTrace, after.CapabilityTrace),
		OriginAnchorsPreserved:   equalStrings(sortUnique(before.ProvenanceAnchors), sortUnique(after.ProvenanceAnchors)),
		Reason:                   "ALL_DECLARED_PROOF_OBLIGATIONS_CLOSED",
	}
	if !outputEqual(expected, before) {
		proof.State, proof.Reason = Refuted, "BASELINE_GENERATED_OUTPUT_MISMATCH"
	} else if !proof.TypePreserved {
		proof.State, proof.Reason = Refuted, "TYPE_CHANGED"
	} else if !proof.EffectPreserved {
		proof.State, proof.Reason = Refuted, "EFFECT_CHANGED"
	} else if !proof.CapabilityPreserved {
		proof.State, proof.Reason = Refuted, "CAPABILITY_CHANGED"
	} else if !proof.TerminalReasonPreserved {
		proof.State, proof.Reason = Refuted, "TERMINAL_REASON_CHANGED"
	} else if !proof.EffectTracePreserved {
		proof.State, proof.Reason = Refuted, "ORDERED_EFFECT_TRACE_CHANGED"
	} else if !proof.CapabilityTracePreserved {
		proof.State, proof.Reason = Refuted, "CAPABILITY_TRACE_CHANGED"
	} else if !proof.OriginAnchorsPreserved {
		proof.State, proof.Reason = Refuted, "PROVENANCE_ANCHOR_SET_CHANGED"
	} else if !outputEqual(before, after) {
		proof.State, proof.Reason = Refuted, "GENERATED_OUTPUT_OBSERVABLE_TUPLE_CHANGED"
	}
	return proof
}

func proofRefutations(proof ProofReport) []Refutation {
	if proof.State != Refuted {
		return nil
	}
	return []Refutation{{Kind: proof.Reason, Reason: proof.Reason}}
}

func metricOf(evidence VariantEvidence) MetricVector {
	return MetricVector{BinaryBytes: evidence.BinaryBytes, GeneratedBytes: evidence.GeneratedBytes, WallMS: evidence.WallMS, PeakRSSKiB: evidence.PeakRSSKiB}
}

func samePair(identity PairIdentity) bool {
	return identity.ScenarioID != "" && identity.SourceDigest != "" && identity.ContractDigest != "" && identity.ToolchainDigest != "" && identity.RunnerDigest != ""
}

func buildAndRun(caseDir, variant string, expr *Expr, metadata generatedMetadata) (VariantEvidence, error) {
	variantDir := filepath.Join(caseDir, variant)
	generatedDir := filepath.Join(variantDir, "generated")
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		return VariantEvidence{}, err
	}
	code, err := GenerateGo(expr, metadata)
	if err != nil {
		return VariantEvidence{}, err
	}
	generatedPath := filepath.Join(generatedDir, "main.go")
	if err := os.WriteFile(generatedPath, code, 0o644); err != nil {
		return VariantEvidence{}, err
	}
	moduleName := "generated." + strings.ReplaceAll(filepath.Base(caseDir), "-", ".") + "." + variant
	if err := os.WriteFile(filepath.Join(generatedDir, "go.mod"), []byte("module "+moduleName+"\n\ngo 1.27.0\n"), 0o644); err != nil {
		return VariantEvidence{}, err
	}
	binaryPath := filepath.Join(generatedDir, "program")
	build, err := measuredCommand(generatedDir, binaryPath, "go", "build", "-trimpath", "-o", binaryPath, ".")
	if err != nil {
		return VariantEvidence{}, fmt.Errorf("go build: %w", err)
	}
	run, err := measuredCommand(generatedDir, binaryPath, binaryPath)
	if err != nil {
		return VariantEvidence{}, fmt.Errorf("generated program: %w", err)
	}
	var output RuntimeResult
	decoder := json.NewDecoder(bytes.NewReader(run.Stdout))
	if err := decoder.Decode(&output); err != nil {
		return VariantEvidence{}, fmt.Errorf("decode generated output: %w; stdout=%q", err, run.Stdout)
	}
	stat, err := os.Stat(binaryPath)
	if err != nil {
		return VariantEvidence{}, err
	}
	return VariantEvidence{GeneratedPath: generatedPath, BinaryPath: binaryPath, GeneratedBytes: int64(len(code)), BinaryBytes: stat.Size(), WallMS: build.WallMS + run.WallMS, PeakRSSKiB: maxInt64(build.PeakRSSKiB, run.PeakRSSKiB), BuildWallMS: build.WallMS, RunWallMS: run.WallMS, BuildRSSKiB: build.PeakRSSKiB, RunRSSKiB: run.PeakRSSKiB, Output: output}, nil
}

type commandObservation struct {
	Stdout     []byte
	WallMS     int64
	PeakRSSKiB int64
}

func measuredCommand(dir, binaryPath, command string, args ...string) (commandObservation, error) {
	rssFile, err := os.CreateTemp(dir, ".rss-")
	if err != nil {
		return commandObservation{}, err
	}
	rssPath := rssFile.Name()
	if err := rssFile.Close(); err != nil {
		return commandObservation{}, err
	}
	defer os.Remove(rssPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	argv := append([]string{"-f", "%M", "-o", rssPath, command}, args...)
	cmd := exec.CommandContext(ctx, "/usr/bin/time", argv...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(dir, ".cache"), "GOTOOLCHAIN=local")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err = cmd.Run()
	wall := time.Since(start).Milliseconds()
	if err != nil {
		return commandObservation{}, fmt.Errorf("%w: stderr=%s", err, strings.TrimSpace(stderr.String()))
	}
	rssRaw, err := os.ReadFile(rssPath)
	if err != nil {
		return commandObservation{}, err
	}
	rss, err := strconv.ParseInt(strings.TrimSpace(string(rssRaw)), 10, 64)
	if err != nil {
		return commandObservation{}, fmt.Errorf("parse peak rss: %w", err)
	}
	return commandObservation{Stdout: stdout.Bytes(), WallMS: wall, PeakRSSKiB: rss}, nil
}

func digestToolchain() (string, error) {
	data, err := exec.Command("go", "version").Output()
	if err != nil {
		return "", err
	}
	material := string(data) + "|" + runtime.GOOS + "|" + runtime.GOARCH
	return Digest([]byte(material)), nil
}

func digestRunner() string {
	material := strings.Join([]string{os.Getenv("RUNNER_OS"), os.Getenv("RUNNER_ARCH"), os.Getenv("ImageOS"), os.Getenv("ImageVersion"), runtime.GOOS, runtime.GOARCH}, "|")
	return Digest([]byte(material))
}

func pathInside(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return true
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func commentOnlySource(source string) string {
	return "// comment-only semantic variant\n\n" + strings.ReplaceAll(strings.ReplaceAll(source, "  ", "\t"), "// Gooo is the authority.", "// Gooo authority comment moved")
}

func readGenerated(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func RenderMarkdown(report ConformanceReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Proof-preserving IR optimizer conformance\n\nDecision: `%s`\n\nFixed denominator: `%d`\n\n", report.Decision, report.FixedDenominator)
	fmt.Fprintf(&b, "Cases: total=%d closed=%d unknown=%d refuted=%d\n\n", report.Summary.TotalCases, report.Summary.Closed, report.Summary.Unknown, report.Summary.Refuted)
	b.WriteString("| case | expected | observed | before generated/binary bytes | after generated/binary bytes | before wall/rss | after wall/rss | proof |\n|---|---|---|---:|---:|---:|---:|---|\n")
	for _, item := range report.Cases {
		fmt.Fprintf(&b, "| %s | %s | %s | %d/%d | %d/%d | %d/%d | %d/%d | %s |\n", item.ID, item.Expected, item.Observed, item.Before.GeneratedBytes, item.Before.BinaryBytes, item.After.GeneratedBytes, item.After.BinaryBytes, item.Before.WallMS, item.Before.PeakRSSKiB, item.After.WallMS, item.After.PeakRSSKiB, item.Proof.Reason)
	}
	b.WriteString("\nResolution precedence: `REFUTED > UNKNOWN > CLOSED`\n")
	return b.String()
}
