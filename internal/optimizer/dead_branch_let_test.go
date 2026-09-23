package optimizer

import "testing"

func TestDeadBranchTraversesLetExpressions(t *testing.T) {
	expr := &Expr{
		Kind: "let",
		Name: "cached",
		ValueExpr: &Expr{
			Kind:       "const-int",
			Type:       "Int",
			Effect:     "PURE",
			Capability: "NONE",
			IntValue:   1,
		},
		BodyExpr: &Expr{
			Kind: "if",
			Args: []*Expr{
				{Kind: "const-bool", Type: "Bool", Effect: "PURE", Capability: "NONE", BoolValue: true, HasBool: true},
				{Kind: "const-int", Type: "Int", Effect: "PURE", Capability: "NONE", IntValue: 2},
				{Kind: "const-int", Type: "Int", Effect: "PURE", Capability: "NONE", IntValue: 3},
			},
		},
	}

	optimized, records, err := deadBranch(expr)
	if err != nil {
		t.Fatalf("dead branch optimization failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if optimized.BodyExpr == nil || optimized.BodyExpr.Kind != "const-int" || optimized.BodyExpr.IntValue != 2 {
		t.Fatalf("let body was not optimized: %#v", optimized.BodyExpr)
	}
}
