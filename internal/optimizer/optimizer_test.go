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
