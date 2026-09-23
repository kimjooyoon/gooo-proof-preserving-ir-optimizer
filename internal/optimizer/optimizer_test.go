package optimizer

import (
	"os"
	"testing"
)

func TestDeclaredOperatorsPreserveSemanticMetadata(t *testing.T) {
	raw, err := os.ReadFile("../../.gooo/proof-preserving-ir-optimizer.gooo")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := ParseContract(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if contract.DenominatorCount != 8 || len(contract.Rewrites) != 3 || len(contract.ProofObligations) != 7 {
		t.Fatalf("contract declarations were not retained: %+v", contract)
	}
}

func TestConstantFoldPreservesSourceAnchors(t *testing.T) {
	expr, err := parseExpr("add(const(2),mul(const(3),const(4)))")
	if err != nil {
		t.Fatal(err)
	}
	assignOrigins(expr, "source.test", "root")
	optimized, records, err := Optimize(expr, "constant-fold")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 || optimized.Kind != "const-int" || optimized.IntValue != 14 {
		t.Fatalf("constant fold did not apply: expr=%s records=%v", optimized.String(), records)
	}
	if !equalStrings(collectAllAnchors(expr), collectAllAnchors(optimized)) {
		t.Fatalf("origin anchors changed: before=%v after=%v", collectAllAnchors(expr), collectAllAnchors(optimized))
	}
}

func TestDeadBranchRecursesThroughLetExpressions(t *testing.T) {
	expr := &Expr{
		Kind: "let",
		Name: "x",
		ValueExpr: &Expr{Kind: "if", Args: []*Expr{
			{Kind: "const-bool", BoolValue: true},
			{Kind: "const-int", IntValue: 1},
			{Kind: "const-int", IntValue: 2},
		}},
		BodyExpr: &Expr{Kind: "if", Args: []*Expr{
			{Kind: "const-bool", BoolValue: false},
			{Kind: "var", Name: "x"},
			{Kind: "const-int", IntValue: 3},
		}},
	}

	optimized, records, err := Optimize(expr, "dead-branch")
	if err != nil {
		t.Fatal(err)
	}
	if optimized.ValueExpr == nil || optimized.ValueExpr.Kind != "const-int" || optimized.ValueExpr.IntValue != 1 {
		t.Fatalf("dead branch was not removed from let value: %+v", optimized.ValueExpr)
	}
	if optimized.BodyExpr == nil || optimized.BodyExpr.Kind != "const-int" || optimized.BodyExpr.IntValue != 3 {
		t.Fatalf("dead branch was not removed from let body: %+v", optimized.BodyExpr)
	}
	if len(records) != 2 {
		t.Fatalf("rewrite record count = %d, want 2", len(records))
	}
}
