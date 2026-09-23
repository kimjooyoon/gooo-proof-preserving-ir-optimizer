package optimizer

import (
	"strings"
	"testing"
)

func TestGenerateGoScopesTopLevelLetWithoutGeneratedNameCollision(t *testing.T) {
	expr := &Expr{
		Kind: "let",
		Name: "effectTrace",
		ValueExpr: &Expr{
			Kind:     "const-int",
			IntValue: 7,
		},
		BodyExpr: &Expr{
			Kind: "var",
			Name: "effectTrace",
		},
	}
	code, err := GenerateGo(expr, generatedMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(code), "\teffectTrace := int64(7)\n") {
		t.Fatal("top-level let must not redeclare a generated tracing local in main")
	}
}
