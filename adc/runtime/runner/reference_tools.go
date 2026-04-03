package runner

import (
	"strings"

	"adjudication/adc/runtime/spec"
)

var referenceToolSet = map[string]bool{
	"get_case":            true,
	"explain_decisions":   true,
	"list_case_files":     true,
	"read_case_text_file": true,
	"request_case_file":   true,
	"get_juror_context":   true,
}

func isReferenceTool(name string) bool {
	return referenceToolSet[strings.TrimSpace(name)]
}

func referenceToolsForRole(role spec.RoleSpec) []string {
	allowed := role.EffectiveAllowedActions()
	tools := make([]string, 0, len(allowed))
	for _, name := range allowed {
		name = strings.TrimSpace(name)
		if name == "" || !isReferenceTool(name) {
			continue
		}
		tools = appendIfMissing(tools, name)
	}
	return tools
}

func opportunityCallableTools(role spec.RoleSpec, opportunity leanOpportunity) []string {
	names := append([]string{}, opportunity.AllowedTools...)
	for _, name := range referenceToolsForRole(role) {
		names = appendIfMissing(names, name)
	}
	if opportunity.MayPass {
		names = appendIfMissing(names, "pass_turn")
	}
	return names
}
