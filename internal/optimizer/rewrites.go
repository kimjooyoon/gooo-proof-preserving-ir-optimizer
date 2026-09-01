package optimizer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

func DeriveMetadata(expr *Expr) error {
	if expr == nil {
		return fmt.Errorf("nil expression")
	}
	for _, arg := range expr.Args {
		if err := DeriveMetadata(arg); err != nil {
			return err
		}
	}
	if err := DeriveMetadata(expr.ValueExpr); expr.ValueExpr != nil && err != nil {
		return err
	}
	if err := DeriveMetadata(expr.BodyExpr); expr.BodyExpr != nil && err != nil {
		return err
	}
	switch expr.Kind {
	case "const-int":
		expr.Type, expr.Effect, expr.Capability = "Int", "PURE", "NONE"
	case "const-bool":
		expr.Type, expr.Effect, expr.Capability = "Bool", "PURE", "NONE"
	case "var":
		if expr.Type == "" {
			expr.Type = "Int"
		}
		expr.Effect, expr.Capability = "PURE", "NONE"
	case "effect":
		if len(expr.Args) != 1 {
			return fmt.Errorf("effect node has %d operands", len(expr.Args))
		}
		expr.Type = expr.Args[0].Type
		expr.Effect, expr.Capability = "IO", "OUTPUT"
	case "add", "mul", "sub":
		if len(expr.Args) != 2 || expr.Args[0].Type != "Int" || expr.Args[1].Type != "Int" {
			return fmt.Errorf("%s node is not a typed Int binary operation", expr.Kind)
		}
		expr.Type = "Int"
		expr.Effect, expr.Capability = joinEffect(expr.Args[0].Effect, expr.Args[1].Effect), joinCapability(expr.Args[0].Capability, expr.Args[1].Capability)
	case "eq":
		if len(expr.Args) != 2 || expr.Args[0].Type != "Int" || expr.Args[1].Type != "Int" {
			return fmt.Errorf("eq node is not a typed Int comparison")
		}
		expr.Type = "Bool"
		expr.Effect, expr.Capability = joinEffect(expr.Args[0].Effect, expr.Args[1].Effect), joinCapability(expr.Args[0].Capability, expr.Args[1].Capability)
	case "if":
		if len(expr.Args) != 3 || expr.Args[0].Type != "Bool" || expr.Args[1].Type != expr.Args[2].Type {
			return fmt.Errorf("if node has incompatible types")
		}
		expr.Type = expr.Args[1].Type
		expr.Effect = joinEffect(expr.Args[0].Effect, joinEffect(expr.Args[1].Effect, expr.Args[2].Effect))
		expr.Capability = joinCapability(expr.Args[0].Capability, joinCapability(expr.Args[1].Capability, expr.Args[2].Capability))
	case "let":
		if expr.ValueExpr == nil || expr.BodyExpr == nil || expr.ValueExpr.Type != expr.BodyExpr.Type {
			return fmt.Errorf("let node has incompatible types")
		}
		expr.Type = expr.BodyExpr.Type
		expr.Effect = joinEffect(expr.ValueExpr.Effect, expr.BodyExpr.Effect)
		expr.Capability = joinCapability(expr.ValueExpr.Capability, expr.BodyExpr.Capability)
	default:
		return fmt.Errorf("unknown IR node kind %q", expr.Kind)
	}
	return nil
}

func joinEffect(left, right string) string {
	if left == "NONDETERMINISTIC" || right == "NONDETERMINISTIC" {
		return "NONDETERMINISTIC"
	}
	if left == "CAPABILITY" || right == "CAPABILITY" {
		return "CAPABILITY"
	}
	if left == "IO" || right == "IO" {
		return "IO"
	}
	return "PURE"
}

func joinCapability(left, right string) string {
	if left == "AUTHORITY" || right == "AUTHORITY" {
		return "AUTHORITY"
	}
	if left == "OUTPUT" || right == "OUTPUT" {
		return "OUTPUT"
	}
	return "NONE"
}

func Optimize(expr *Expr, operator string) (*Expr, []RewriteRecord, error) {
	if err := DeriveMetadata(expr); err != nil {
		return nil, nil, err
	}
	switch operator {
	case "constant-fold":
		return constantFold(expr.Clone())
	case "dead-branch":
		return deadBranch(expr.Clone())
	case "deterministic-cse":
		return deterministicCSE(expr.Clone())
	default:
		return nil, nil, fmt.Errorf("operator %q is not declared in .gooo", operator)
	}
}

func constantFold(expr *Expr) (*Expr, []RewriteRecord, error) {
	if expr == nil {
		return nil, nil, fmt.Errorf("constant folding received nil IR")
	}
	records := []RewriteRecord{}
	for index, arg := range expr.Args {
		optimized, childRecords, err := constantFold(arg)
		if err != nil {
			return nil, nil, err
		}
		expr.Args[index] = optimized
		records = append(records, childRecords...)
	}
	if expr.ValueExpr != nil {
		optimized, childRecords, err := constantFold(expr.ValueExpr)
		if err != nil {
			return nil, nil, err
		}
		expr.ValueExpr = optimized
		records = append(records, childRecords...)
	}
	if expr.BodyExpr != nil {
		optimized, childRecords, err := constantFold(expr.BodyExpr)
		if err != nil {
			return nil, nil, err
		}
		expr.BodyExpr = optimized
		records = append(records, childRecords...)
	}
	if folded, ok := foldPure(expr); ok {
		records = append(records, RewriteRecord{Operator: "constant-fold", Rule: "fold-pure-int-and-bool", Before: []string{expr.String()}, After: []string{folded.String()}})
		return folded, records, nil
	}
	if err := DeriveMetadata(expr); err != nil {
		return nil, nil, err
	}
	return expr, records, nil
}

func foldPure(expr *Expr) (*Expr, bool) {
	if expr.Effect != "PURE" || expr.Capability != "NONE" || len(expr.Args) != 2 {
		return nil, false
	}
	left, right := expr.Args[0], expr.Args[1]
	result := &Expr{Origin: "derived/constant-fold", Anchors: collectOrigins(expr)}
	switch expr.Kind {
	case "add", "mul", "sub":
		if left.Kind != "const-int" || right.Kind != "const-int" {
			return nil, false
		}
		result.Kind, result.Type, result.Effect, result.Capability = "const-int", "Int", "PURE", "NONE"
		switch expr.Kind {
		case "add":
			result.IntValue = left.IntValue + right.IntValue
		case "mul":
			result.IntValue = left.IntValue * right.IntValue
		case "sub":
			result.IntValue = left.IntValue - right.IntValue
		}
	case "eq":
		if left.Kind != "const-int" || right.Kind != "const-int" {
			return nil, false
		}
		result.Kind, result.Type, result.Effect, result.Capability, result.HasBool = "const-bool", "Bool", "PURE", "NONE", true
		result.BoolValue = left.IntValue == right.IntValue
	default:
		return nil, false
	}
	return result, true
}

func deadBranch(expr *Expr) (*Expr, []RewriteRecord, error) {
	if expr == nil {
		return nil, nil, fmt.Errorf("dead branch elimination received nil IR")
	}
	records := []RewriteRecord{}
	for index, arg := range expr.Args {
		optimized, childRecords, err := deadBranch(arg)
		if err != nil {
			return nil, nil, err
		}
		expr.Args[index] = optimized
		records = append(records, childRecords...)
	}
	if expr.Kind == "if" && len(expr.Args) == 3 && expr.Args[0].Kind == "const-bool" {
		discarded := expr.Args[2]
		selected := expr.Args[1]
		if !expr.Args[0].BoolValue {
			discarded, selected = expr.Args[1], expr.Args[2]
		}
		if discarded.Effect == "PURE" && discarded.Capability == "NONE" {
			result := selected.Clone()
			result.Anchors = mergeStrings(collectOrigins(expr), collectOrigins(selected))
			records = append(records, RewriteRecord{Operator: "dead-branch", Rule: "remove-constant-condition-only-when-discarded-branch-is-effect-free", Before: []string{expr.String()}, After: []string{result.String()}})
			if err := DeriveMetadata(result); err != nil {
				return nil, nil, err
			}
			return result, records, nil
		}
	}
	if err := DeriveMetadata(expr); err != nil {
		return nil, nil, err
	}
	return expr, records, nil
}

func unsafeDeadBranch(expr *Expr) (*Expr, error) {
	if expr == nil {
		return nil, fmt.Errorf("unsafe dead branch elimination received nil IR")
	}
	for index, arg := range expr.Args {
		optimized, err := unsafeDeadBranch(arg)
		if err != nil {
			return nil, err
		}
		expr.Args[index] = optimized
	}
	if expr.Kind == "if" && len(expr.Args) == 3 && expr.Args[0].Kind == "const-bool" {
		// This path is intentionally adversarial. It models a candidate that
		// removes a branch without proving the declared condition/effect rule.
		selected := expr.Args[1]
		result := selected.Clone()
		result.Anchors = mergeStrings(collectOrigins(expr), collectOrigins(selected))
		if err := DeriveMetadata(result); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := DeriveMetadata(expr); err != nil {
		return nil, err
	}
	return expr, nil
}

func deterministicCSE(expr *Expr) (*Expr, []RewriteRecord, error) {
	if expr == nil {
		return nil, nil, fmt.Errorf("CSE received nil IR")
	}
	rewritten := expr.Clone()
	counts := map[string]int{}
	var first *Expr
	var firstKey string
	var visit func(*Expr)
	visit = func(node *Expr) {
		if node == nil {
			return
		}
		for _, arg := range node.Args {
			visit(arg)
		}
		visit(node.ValueExpr)
		visit(node.BodyExpr)
		if node.Effect == "PURE" && node.Capability == "NONE" && node.Kind != "const-int" && node.Kind != "const-bool" && node.Kind != "var" && node.Kind != "let" {
			key := node.String()
			counts[key]++
			if first == nil && counts[key] == 2 {
				first = node
				firstKey = key
			}
		}
	}
	visit(rewritten)
	if first == nil {
		if err := DeriveMetadata(rewritten); err != nil {
			return nil, nil, err
		}
		return rewritten, nil, nil
	}
	constName := "cse0"
	body := replaceCSE(rewritten, firstKey, constName, true)
	result := &Expr{Kind: "let", Name: constName, ValueExpr: first.Clone(), BodyExpr: body, Origin: "derived/deterministic-cse", Anchors: mergeStrings(collectOrigins(rewritten))}
	if err := DeriveMetadata(result); err != nil {
		return nil, nil, err
	}
	record := RewriteRecord{Operator: "deterministic-cse", Rule: "reuse-first-left-to-right-pure-repeated-subexpression", Before: []string{rewritten.String()}, After: []string{result.String()}}
	return result, []RewriteRecord{record}, nil
}

func replaceCSE(node *Expr, key, name string, replace bool) *Expr {
	if node == nil {
		return nil
	}
	if replace && node.String() == key {
		result := &Expr{Kind: "var", Name: name, Type: node.Type, Effect: "PURE", Capability: "NONE", Origin: node.Origin, Anchors: collectOrigins(node)}
		return result
	}
	result := node.Clone()
	for index, arg := range result.Args {
		result.Args[index] = replaceCSE(arg, key, name, replace)
	}
	result.ValueExpr = replaceCSE(result.ValueExpr, key, name, replace)
	result.BodyExpr = replaceCSE(result.BodyExpr, key, name, replace)
	return result
}

func collectAllAnchors(expr *Expr) []string {
	return sortUnique(collectOrigins(expr))
}

func sortUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func expressionDigest(expr *Expr) string {
	sum := sha256.Sum256([]byte(expr.Canonical()))
	return "sha256:" + hex.EncodeToString(sum[:])
}
