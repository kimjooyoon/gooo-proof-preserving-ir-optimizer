package optimizer

import (
	"encoding/json"
	"fmt"
	"os"
)

type evalValue struct {
	intValue  int64
	boolValue bool
	isBool    bool
}

func Evaluate(expr *Expr) (RuntimeResult, error) {
	value, effects, capabilities, err := eval(expr, map[string]evalValue{})
	if err != nil {
		return RuntimeResult{}, err
	}
	result := RuntimeResult{
		Value:             value.intValue,
		TerminalReason:    terminalReason(effects),
		Effect:            expr.Effect,
		Capability:        expr.Capability,
		EffectTrace:       effects,
		CapabilityTrace:   capabilities,
		ProvenanceAnchors: collectAllAnchors(expr),
	}
	if value.isBool {
		return RuntimeResult{}, fmt.Errorf("top-level program produced Bool; generated entrypoint requires Int")
	}
	return result, nil
}

func eval(expr *Expr, env map[string]evalValue) (evalValue, []string, []string, error) {
	if expr == nil {
		return evalValue{}, nil, nil, fmt.Errorf("cannot evaluate nil expression")
	}
	switch expr.Kind {
	case "const-int":
		return evalValue{intValue: expr.IntValue}, nil, nil, nil
	case "const-bool":
		return evalValue{boolValue: expr.BoolValue, isBool: true}, nil, nil, nil
	case "var":
		value, ok := env[expr.Name]
		if !ok {
			return evalValue{}, nil, nil, fmt.Errorf("unbound variable %q", expr.Name)
		}
		return value, nil, nil, nil
	case "effect":
		value, effects, capabilities, err := eval(expr.Args[0], env)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		effects = append([]string{expr.Name}, effects...)
		capabilities = append([]string{"OUTPUT"}, capabilities...)
		return value, effects, capabilities, nil
	case "add", "mul", "sub", "eq":
		left, leftEffects, leftCapabilities, err := eval(expr.Args[0], env)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		right, rightEffects, rightCapabilities, err := eval(expr.Args[1], env)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		value := evalValue{}
		switch expr.Kind {
		case "add":
			value.intValue = left.intValue + right.intValue
		case "mul":
			value.intValue = left.intValue * right.intValue
		case "sub":
			value.intValue = left.intValue - right.intValue
		case "eq":
			value.boolValue, value.isBool = left.intValue == right.intValue, true
		}
		return value, append(leftEffects, rightEffects...), append(leftCapabilities, rightCapabilities...), nil
	case "if":
		condition, conditionEffects, conditionCapabilities, err := eval(expr.Args[0], env)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		branch := expr.Args[2]
		if condition.boolValue {
			branch = expr.Args[1]
		}
		value, branchEffects, branchCapabilities, err := eval(branch, env)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		return value, append(conditionEffects, branchEffects...), append(conditionCapabilities, branchCapabilities...), nil
	case "let":
		value, valueEffects, valueCapabilities, err := eval(expr.ValueExpr, env)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		childEnv := make(map[string]evalValue, len(env)+1)
		for key, item := range env {
			childEnv[key] = item
		}
		childEnv[expr.Name] = value
		bodyValue, bodyEffects, bodyCapabilities, err := eval(expr.BodyExpr, childEnv)
		if err != nil {
			return evalValue{}, nil, nil, err
		}
		return bodyValue, append(valueEffects, bodyEffects...), append(valueCapabilities, bodyCapabilities...), nil
	default:
		return evalValue{}, nil, nil, fmt.Errorf("cannot evaluate IR node %q", expr.Kind)
	}
}

func terminalReason(effects []string) string {
	if len(effects) == 0 {
		return "RETURN"
	}
	return "RETURN_AFTER_EFFECT"
}

type generatedMetadata struct {
	Effect            string
	Capability        string
	TerminalReason    string
	ProvenanceAnchors []string
}

func GenerateGo(expr *Expr, metadata generatedMetadata) ([]byte, error) {
	if err := DeriveMetadata(expr); err != nil {
		return nil, err
	}
	if metadata.Effect == "" {
		metadata.Effect = expr.Effect
	}
	if metadata.Capability == "" {
		metadata.Capability = expr.Capability
	}
	if metadata.TerminalReason == "" {
		metadata.TerminalReason = terminalReasonFromExpr(expr)
	}
	if metadata.ProvenanceAnchors == nil {
		metadata.ProvenanceAnchors = collectAllAnchors(expr)
	}
	valueExpr, err := renderInt(expr, map[string]string{})
	if err != nil {
		return nil, err
	}
	anchorsJSON, err := json.Marshal(metadata.ProvenanceAnchors)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
)

type proofOutput struct {
	Value             int64    %s
	TerminalReason    string   %s
	Effect            string   %s
	Capability        string   %s
	EffectTrace       []string %s
	CapabilityTrace   []string %s
	ProvenanceAnchors []string %s
}

func main() {
	effectTrace := []string{}
	capabilityTrace := []string{}
	value := int64(%s)
	out := proofOutput{
		Value: value,
		TerminalReason: %q,
		Effect: %q,
		Capability: %q,
		EffectTrace: effectTrace,
		CapabilityTrace: capabilityTrace,
		ProvenanceAnchors: %s,
	}
	data, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}
`, "`json:\"value\"`", "`json:\"terminal_reason\"`", "`json:\"effect\"`", "`json:\"capability\"`", "`json:\"effect_trace\"`", "`json:\"capability_trace\"`", "`json:\"provenance_anchors\"`", valueExpr, metadata.TerminalReason, metadata.Effect, metadata.Capability, anchorsJSON)), nil
}

func terminalReasonFromExpr(expr *Expr) string {
	if expr.Effect == "PURE" {
		return "RETURN"
	}
	return "RETURN_AFTER_EFFECT"
}

func renderInt(expr *Expr, env map[string]string) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("cannot generate nil expression")
	}
	switch expr.Kind {
	case "const-int":
		return fmt.Sprintf("%d", expr.IntValue), nil
	case "var":
		name, ok := env[expr.Name]
		if !ok {
			return "", fmt.Errorf("cannot generate unbound variable %q", expr.Name)
		}
		return name, nil
	case "add", "mul", "sub":
		left, err := renderInt(expr.Args[0], env)
		if err != nil {
			return "", err
		}
		right, err := renderInt(expr.Args[1], env)
		if err != nil {
			return "", err
		}
		op := map[string]string{"add": "+", "mul": "*", "sub": "-"}[expr.Kind]
		return "(" + left + " " + op + " " + right + ")", nil
	case "effect":
		child, err := renderInt(expr.Args[0], env)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(func() int64 { effectTrace = append(effectTrace, %q); capabilityTrace = append(capabilityTrace, %q); return %s }())", expr.Name, "OUTPUT", child), nil
	case "if":
		condition, err := renderBool(expr.Args[0], env)
		if err != nil {
			return "", err
		}
		thenValue, err := renderInt(expr.Args[1], env)
		if err != nil {
			return "", err
		}
		elseValue, err := renderInt(expr.Args[2], env)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(func() int64 { if %s { return %s }; return %s }())", condition, thenValue, elseValue), nil
	case "let":
		value, err := renderInt(expr.ValueExpr, env)
		if err != nil {
			return "", err
		}
		childEnv := make(map[string]string, len(env)+1)
		for key, item := range env {
			childEnv[key] = item
		}
		childEnv[expr.Name] = expr.Name
		body, err := renderInt(expr.BodyExpr, childEnv)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(func() int64 { %s := int64(%s); return %s }())", expr.Name, value, body), nil
	default:
		return "", fmt.Errorf("cannot render %s as Int", expr.Kind)
	}
}

func renderBool(expr *Expr, env map[string]string) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("cannot generate nil Boolean expression")
	}
	switch expr.Kind {
	case "const-bool":
		if expr.BoolValue {
			return "true", nil
		}
		return "false", nil
	case "eq":
		left, err := renderInt(expr.Args[0], env)
		if err != nil {
			return "", err
		}
		right, err := renderInt(expr.Args[1], env)
		if err != nil {
			return "", err
		}
		return "(" + left + " == " + right + ")", nil
	default:
		return "", fmt.Errorf("cannot render %s as Bool", expr.Kind)
	}
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
