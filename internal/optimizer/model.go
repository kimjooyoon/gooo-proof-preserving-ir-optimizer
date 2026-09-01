package optimizer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Decision string

const (
	Closed  Decision = "CLOSED"
	Unknown Decision = "UNKNOWN"
	Refuted Decision = "REFUTED"
)

type AuthorityRule struct {
	RepositoryWrites int    `json:"repository_writes"`
	OutputScope      string `json:"output_scope"`
	AutomaticCommit  int    `json:"automatic_commit"`
	AutomaticPush    int    `json:"automatic_push"`
	AutomaticMerge   int    `json:"automatic_merge"`
	AutomaticRelease int    `json:"automatic_release"`
}

type TypeDecl struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type EffectDecl struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type CapabilityDecl struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type ForbiddenEffect struct {
	Effect string `json:"effect"`
	When   string `json:"when"`
}

type OriginMapDecl struct {
	Input        string `json:"input"`
	Output       string `json:"output"`
	Preservation string `json:"preservation"`
}

type RewriteRule struct {
	Name      string `json:"name"`
	Rule      string `json:"rule"`
	Requires  string `json:"requires"`
	Preserves string `json:"preserves"`
}

type ProofObligation struct {
	ID      string `json:"id"`
	Stage   string `json:"stage"`
	Step    string `json:"step"`
	Proof   string `json:"proof"`
	Missing string `json:"missing"`
}

type CostPolicy struct {
	Vector string `json:"vector"`
	Pair   string `json:"pair"`
	Policy string `json:"policy"`
}

type Scenario struct {
	ID       string
	Expected Decision
	Source   string
	Operator string
	Program  string
	Variant  string
	Replay   bool
}

type ProgramDecl struct {
	ID         string
	ExprText   string
	Type       string
	Effect     string
	Capability string
	Result     int64
	Reason     string
	Origin     string
	Expr       *Expr `json:"expr,omitempty"`
}

type Contract struct {
	Schema             string
	Authority          string
	Grammar            []string
	Version            string
	Language           string
	Precedence         []Decision
	UnknownFields      []string
	DenominatorID      string
	DenominatorCount   int
	Normalization      map[string]string
	Types              []TypeDecl
	Effects            []EffectDecl
	Capabilities       []CapabilityDecl
	ForbiddenEffects   []ForbiddenEffect
	OriginMap          OriginMapDecl
	Rewrites           []RewriteRule
	ProofObligations   []ProofObligation
	Cost               CostPolicy
	AuthorityRule      AuthorityRule
	Programs           []ProgramDecl
	Scenarios          []Scenario
	SourceDigest       string
	SourceCanonicalKey string
}

type Expr struct {
	Kind       string   `json:"kind"`
	Type       string   `json:"type"`
	Effect     string   `json:"effect"`
	Capability string   `json:"capability"`
	Origin     string   `json:"origin"`
	Anchors    []string `json:"anchors"`
	Name       string   `json:"name,omitempty"`
	IntValue   int64    `json:"int_value,omitempty"`
	BoolValue  bool     `json:"bool_value,omitempty"`
	HasBool    bool     `json:"has_bool,omitempty"`
	Args       []*Expr  `json:"args,omitempty"`
	ValueExpr  *Expr    `json:"value_expr,omitempty"`
	BodyExpr   *Expr    `json:"body_expr,omitempty"`
}

type RewriteRecord struct {
	Operator string   `json:"operator"`
	Rule     string   `json:"rule"`
	Before   []string `json:"before"`
	After    []string `json:"after"`
}

type RuntimeResult struct {
	Value             int64    `json:"value"`
	TerminalReason    string   `json:"terminal_reason"`
	Effect            string   `json:"effect"`
	Capability        string   `json:"capability"`
	EffectTrace       []string `json:"effect_trace"`
	CapabilityTrace   []string `json:"capability_trace"`
	ProvenanceAnchors []string `json:"provenance_anchors"`
}

type MetricVector struct {
	BinaryBytes    int64 `json:"binary_bytes"`
	GeneratedBytes int64 `json:"generated_bytes"`
	WallMS         int64 `json:"wall_ms"`
	PeakRSSKiB     int64 `json:"peak_rss_kib"`
}

type VariantEvidence struct {
	GeneratedPath  string        `json:"generated_path"`
	BinaryPath     string        `json:"binary_path"`
	GeneratedBytes int64         `json:"generated_bytes"`
	BinaryBytes    int64         `json:"binary_bytes"`
	WallMS         int64         `json:"wall_ms"`
	PeakRSSKiB     int64         `json:"peak_rss_kib"`
	BuildWallMS    int64         `json:"build_wall_ms"`
	RunWallMS      int64         `json:"run_wall_ms"`
	BuildRSSKiB    int64         `json:"build_peak_rss_kib"`
	RunRSSKiB      int64         `json:"run_peak_rss_kib"`
	Output         RuntimeResult `json:"output"`
}

type PairIdentity struct {
	ScenarioID      string `json:"scenario_id"`
	SourceDigest    string `json:"source_digest"`
	ContractDigest  string `json:"contract_digest"`
	ToolchainDigest string `json:"toolchain_digest"`
	RunnerDigest    string `json:"runner_digest"`
}

type ProofReport struct {
	State                    Decision `json:"state"`
	TypePreserved            bool     `json:"type_preserved"`
	EffectPreserved          bool     `json:"effect_preserved"`
	CapabilityPreserved      bool     `json:"capability_preserved"`
	TerminalReasonPreserved  bool     `json:"terminal_reason_preserved"`
	EffectTracePreserved     bool     `json:"ordered_effect_trace_preserved"`
	CapabilityTracePreserved bool     `json:"capability_trace_preserved"`
	OriginAnchorsPreserved   bool     `json:"provenance_anchors_preserved"`
	Reason                   string   `json:"reason"`
}

type UnknownEvidence struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type Refutation struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type ReplayEvidence struct {
	State         Decision `json:"state"`
	GeneratedSame bool     `json:"generated_same"`
	OutputSame    bool     `json:"output_same"`
	TerminalSame  bool     `json:"terminal_same"`
	ReplayDigest  string   `json:"replay_digest"`
}

type ImprovementClaim struct {
	State     Decision     `json:"state"`
	Reason    string       `json:"reason"`
	ExactPair bool         `json:"exact_pair"`
	Before    MetricVector `json:"before"`
	After     MetricVector `json:"after"`
}

type CaseReport struct {
	ID                string           `json:"id"`
	Expected          Decision         `json:"expected"`
	Observed          Decision         `json:"observed"`
	Operator          string           `json:"operator"`
	Variant           string           `json:"variant"`
	SourceDigest      string           `json:"source_digest"`
	SemanticSourceKey string           `json:"semantic_source_key"`
	ContractDigest    string           `json:"contract_digest"`
	ToolchainDigest   string           `json:"toolchain_digest"`
	RunnerDigest      string           `json:"runner_digest"`
	PairIdentity      PairIdentity     `json:"pair_identity"`
	Before            VariantEvidence  `json:"before"`
	After             VariantEvidence  `json:"after"`
	Proof             ProofReport      `json:"proof"`
	CostWitness       bool             `json:"cost_witness"`
	Improvement       ImprovementClaim `json:"improvement"`
	Unknown           *UnknownEvidence `json:"unknown,omitempty"`
	Refutations       []Refutation     `json:"refutations,omitempty"`
	Replay            *ReplayEvidence  `json:"replay,omitempty"`
	RewriteRecords    []RewriteRecord  `json:"rewrite_records"`
}

type Summary struct {
	TotalCases int `json:"total_cases"`
	Closed     int `json:"closed"`
	Unknown    int `json:"unknown"`
	Refuted    int `json:"refuted"`
}

type GeneratedInventory struct {
	GeneratedGoFiles  int64 `json:"generated_go_files"`
	GeneratedGoBytes  int64 `json:"generated_go_bytes"`
	GeneratedBinaries int64 `json:"generated_binaries"`
	BinaryBytes       int64 `json:"binary_bytes"`
}

type ConformanceReport struct {
	Schema             string             `json:"schema"`
	Decision           Decision           `json:"decision"`
	ObservedPrecedence []Decision         `json:"observed_precedence"`
	FixedDenominator   int                `json:"fixed_denominator"`
	SourceDigest       string             `json:"source_digest"`
	ContractDigest     string             `json:"contract_digest"`
	ToolchainDigest    string             `json:"toolchain_digest"`
	RunnerDigest       string             `json:"runner_digest"`
	Summary            Summary            `json:"summary"`
	Generated          GeneratedInventory `json:"generated"`
	Authority          AuthorityRule      `json:"authority"`
	Cases              []CaseReport       `json:"cases"`
	NoAggregateMetrics bool               `json:"no_aggregate_metrics"`
}

func (e *Expr) Clone() *Expr {
	if e == nil {
		return nil
	}
	c := *e
	c.Anchors = append([]string(nil), e.Anchors...)
	c.Args = make([]*Expr, len(e.Args))
	for i, arg := range e.Args {
		c.Args[i] = arg.Clone()
	}
	c.ValueExpr = e.ValueExpr.Clone()
	c.BodyExpr = e.BodyExpr.Clone()
	return &c
}

func (e *Expr) String() string {
	if e == nil {
		return "<nil>"
	}
	switch e.Kind {
	case "const-int":
		return fmt.Sprintf("const(%d)", e.IntValue)
	case "const-bool":
		return fmt.Sprintf("const(%t)", e.BoolValue)
	case "var":
		return fmt.Sprintf("var(%s)", e.Name)
	case "effect":
		return fmt.Sprintf("effect(%s,%s)", e.Name, e.Args[0].String())
	case "let":
		return fmt.Sprintf("let(%s,%s,%s)", e.Name, e.ValueExpr.String(), e.BodyExpr.String())
	default:
		parts := make([]string, len(e.Args))
		for i, arg := range e.Args {
			parts[i] = arg.String()
		}
		return e.Kind + "(" + strings.Join(parts, ",") + ")"
	}
}

func (e *Expr) Canonical() string {
	return e.String() + "|type=" + e.Type + "|effect=" + e.Effect + "|capability=" + e.Capability + "|anchors=" + strings.Join(sortedStrings(collectOrigins(e)), ",")
}

func sortedStrings(input []string) []string {
	out := append([]string(nil), input...)
	sort.Strings(out)
	return out
}

func mergeStrings(values ...[]string) []string {
	set := map[string]bool{}
	for _, group := range values {
		for _, value := range group {
			if value != "" {
				set[value] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func collectOrigins(e *Expr) []string {
	if e == nil {
		return nil
	}
	result := append([]string(nil), e.Anchors...)
	if e.Origin != "" && !strings.HasPrefix(e.Origin, "derived/") {
		result = append(result, e.Origin)
	}
	for _, arg := range e.Args {
		result = mergeStrings(result, collectOrigins(arg))
	}
	result = mergeStrings(result, collectOrigins(e.ValueExpr), collectOrigins(e.BodyExpr))
	return mergeStrings(result)
}

func outputEqual(left, right RuntimeResult) bool {
	left.ProvenanceAnchors = sortedStrings(left.ProvenanceAnchors)
	right.ProvenanceAnchors = sortedStrings(right.ProvenanceAnchors)
	if len(left.EffectTrace) == 0 {
		left.EffectTrace = nil
	}
	if len(right.EffectTrace) == 0 {
		right.EffectTrace = nil
	}
	if len(left.CapabilityTrace) == 0 {
		left.CapabilityTrace = nil
	}
	if len(right.CapabilityTrace) == 0 {
		right.CapabilityTrace = nil
	}
	if len(left.ProvenanceAnchors) == 0 {
		left.ProvenanceAnchors = nil
	}
	if len(right.ProvenanceAnchors) == 0 {
		right.ProvenanceAnchors = nil
	}
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return string(a) == string(b)
}
