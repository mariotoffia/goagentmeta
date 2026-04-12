package lowering

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// LowerRule transforms a rule into an inline instruction section.
// This is always a safe lowering: rules map cleanly to instruction content.
func LowerRule(obj pipeline.NormalizedObject) pipeline.LoweredObject {
	var sb strings.Builder

	// Emit conditions as "When:" prose preceding the content.
	conditions := extractRuleConditions(obj)
	for _, cond := range conditions {
		sb.WriteString(fmt.Sprintf("When %s = %s:\n", cond.Type, cond.Value))
	}

	// Emit the rule content as a Markdown instruction section.
	sb.WriteString(fmt.Sprintf("## %s\n\n", ruleHeading(obj)))
	if content, ok := obj.ResolvedFields["content"].(string); ok && content != "" {
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}

	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindRule,
		LoweredKind:  model.KindInstruction,
		Decision: pipeline.LoweringDecision{
			Action:       "lowered",
			Reason:       "rule → instruction section (always safe)",
			Preservation: obj.Meta.Preservation,
			Safe:         true,
		},
		Content: sb.String(),
		Fields:  obj.ResolvedFields,
	}
}

// RuleLoweringRecord produces a LoweringRecord for a rule lowering.
func RuleLoweringRecord(obj pipeline.NormalizedObject) pipeline.LoweringRecord {
	return pipeline.LoweringRecord{
		ObjectID:     obj.Meta.ID,
		FromKind:     model.KindRule,
		ToKind:       model.KindInstruction,
		Reason:       "rule → instruction section (always safe)",
		Preservation: obj.Meta.Preservation,
		Status:       "lowered",
	}
}

func extractRuleConditions(obj pipeline.NormalizedObject) []model.RuleCondition {
	raw, ok := obj.ResolvedFields["conditions"]
	if !ok {
		return nil
	}
	condSlice, ok := raw.([]model.RuleCondition)
	if ok {
		return condSlice
	}
	return nil
}

func ruleHeading(obj pipeline.NormalizedObject) string {
	if obj.Meta.Description != "" {
		return obj.Meta.Description
	}
	return obj.Meta.ID
}
