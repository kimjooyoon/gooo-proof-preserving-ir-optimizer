package optimizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExprRejectsUnclosedLeafExpressions(t *testing.T) {
	for _, input := range []string{"const(1", "var(x"} {
		if _, err := parseExpr(input); err == nil {
			t.Fatalf("parseExpr(%q) accepted an unclosed expression", input)
		}
	}
}

func TestParseExprRejectsMalformedVariableNames(t *testing.T) {
	for _, input := range []string{"var(x y)", "var(x,y)", "var()"} {
		if _, err := parseExpr(input); err == nil {
			t.Fatalf("parseExpr(%q) accepted a malformed variable name", input)
		}
	}
}

func TestParseContractRejectsDuplicateSingletonDeclaration(t *testing.T) {
	path := filepath.Join("..", "..", ".gooo", "proof-preserving-ir-optimizer.gooo")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(raw), "\n}\n", "\nauthority metacode\n}\n", 1)
	if text == string(raw) {
		t.Fatal("contract closing brace not found")
	}
	if _, err := ParseContract(text); err == nil {
		t.Fatal("duplicate singleton declaration was accepted")
	}
}
