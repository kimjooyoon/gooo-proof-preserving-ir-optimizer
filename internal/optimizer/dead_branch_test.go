package optimizer

import "testing"

func TestDeadBranchReachesNestedLetValue(t *testing.T) {
	expr := &Expr{
		Kind: "let", Name: "x",
		ValueExpr: &Expr{
			Kind: "if",
			Args: []*Expr{
				{Kind: "const-bool", BoolValue: false},
				{Kind: "add", Args: []*Expr{{Kind: "const-int", IntValue: 1}, {Kind: "const-int", IntValue: 2}}},
				{Kind: "const-int", IntValue: 3},
			},
		},
		BodyExpr: &Expr{Kind: "add", Args: []*Expr{{Kind: "var", Name: "x"}, {Kind: "const-int", IntValue: 4}}},
	}
	optimized, records, err := Optimize(expr, "dead-branch")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 || optimized.ValueExpr == nil || optimized.ValueExpr.Kind != "const-int" || optimized.ValueExpr.IntValue != 3 {
		t.Fatalf("nested dead branch was not removed: optimized=%#v records=%#v", optimized, records)
	}
}
