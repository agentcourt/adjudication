package manage

import (
	"fmt"
	"strconv"
	"strings"
)

// Field describes one create-request field: how the start form renders
// it and how the payload builder types it.  The set mirrors the clerk
// and direct create requests documented in the ARB manual.
type Field struct {
	Name        string // JSON field name, also the form input name
	Label       string
	Group       string
	Type        string   // text, int, list, or select
	Options     []string // select choices, first entry empty for "omit"
	Kinds       []string // clerk, attested, direct
	Attestation bool     // nested under execution.attestation
}

// Kinds offered by the start form.  Attested runs go through the clerk
// API with execution.mode attested; the service rejects the request
// when its attested configuration is missing.
var Kinds = []string{"clerk", "attested", "direct"}

var Fields = []Field{
	{Name: "example", Label: "Example name", Group: "Case", Type: "text", Kinds: []string{"clerk", "attested"}},
	{Name: "complaint_path", Label: "Complaint path", Group: "Case", Type: "text", Kinds: []string{"clerk", "attested", "direct"}},
	{Name: "case_files", Label: "Case files (one per line)", Group: "Case", Type: "list", Kinds: []string{"clerk", "attested", "direct"}},
	{Name: "case_id", Label: "Case id", Group: "Case", Type: "text", Kinds: []string{"clerk", "attested", "direct"}},
	{Name: "run_id", Label: "Run id", Group: "Case", Type: "text", Kinds: []string{"clerk", "attested", "direct"}},
	{Name: "out_dir", Label: "Output directory (child of the service out root)", Group: "Case", Type: "text", Kinds: []string{"clerk", "attested", "direct"}},
	{Name: "policy_path", Label: "Policy path", Group: "Case", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "evidence_standard", Label: "Evidence standard", Group: "Case", Type: "text", Kinds: []string{"clerk"}},

	{Name: "council_backend", Label: "Council backend", Group: "Council", Type: "text", Kinds: []string{"direct"}},
	{Name: "council_size", Label: "Council size", Group: "Council", Type: "int", Kinds: []string{"clerk"}},
	{Name: "council_pool_path", Label: "Council pool path", Group: "Council", Type: "text", Kinds: []string{"clerk"}},
	{Name: "council_timeout_seconds", Label: "Council timeout seconds", Group: "Council", Type: "int", Kinds: []string{"clerk", "direct"}},
	{Name: "council_instructions", Label: "Council instruction template", Group: "Council", Type: "text", Kinds: []string{"clerk"}},
	{Name: "pi_image", Label: "Pi image", Group: "Council", Type: "text", Kinds: []string{"clerk"}},
	{Name: "pi_mcp_adapter", Label: "Pi MCP adapter", Group: "Council", Type: "text", Kinds: []string{"clerk"}},
	{Name: "council_output_limit_bytes", Label: "Council output limit bytes", Group: "Council", Type: "int", Kinds: []string{"clerk"}},

	{Name: "auto_lawyers", Label: "Auto lawyers", Group: "Lawyers", Type: "select", Options: []string{"", "both", "plaintiff", "defendant"}, Kinds: []string{"clerk"}},
	{Name: "attorney_instructions", Label: "Attorney instructions path", Group: "Lawyers", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "prompt_dir", Label: "Prompt directory", Group: "Lawyers", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "attorney_common_prompt", Label: "Attorney common prompt", Group: "Lawyers", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "attorney_arguments_prompt", Label: "Attorney arguments prompt", Group: "Lawyers", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "attorney_rebuttals_prompt", Label: "Attorney rebuttals prompt", Group: "Lawyers", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "lawyer_timeout_seconds", Label: "Lawyer timeout seconds", Group: "Lawyers", Type: "int", Kinds: []string{"clerk", "direct"}},
	{Name: "lawyer_instructions", Label: "OpenClaw lawyer instruction template", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "remote_lawyer_skill", Label: "Remote lawyer skill template", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "mcp_public_base_url", Label: "Public MCP base URL", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "openclaw_auth", Label: "OpenClaw auth", Group: "Lawyers", Type: "select", Options: []string{"", "auto", "codex", "api-key"}, Kinds: []string{"clerk"}},
	{Name: "openclaw_codex_auth_path", Label: "Codex auth.json path", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "openclaw_image", Label: "OpenClaw image", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "openclaw_model", Label: "OpenClaw model", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "openclaw_thinking", Label: "OpenClaw thinking", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "openclaw_timeout_seconds", Label: "OpenClaw timeout seconds", Group: "Lawyers", Type: "int", Kinds: []string{"clerk"}},
	{Name: "openclaw_lawyer_start_delay_seconds", Label: "OpenClaw lawyer start delay seconds", Group: "Lawyers", Type: "int", Kinds: []string{"clerk"}},
	{Name: "docker_command", Label: "Docker command", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "podman_command", Label: "Podman command", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "docker_mcp_host", Label: "Docker MCP host", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},
	{Name: "podman_mcp_host", Label: "Podman MCP host", Group: "Lawyers", Type: "text", Kinds: []string{"clerk"}},

	{Name: "common_root", Label: "Common root", Group: "Runtime", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "engine_path", Label: "Engine path", Group: "Runtime", Type: "text", Kinds: []string{"clerk", "direct"}},
	{Name: "caseapi_addr", Label: "Case API address", Group: "Runtime", Type: "text", Kinds: []string{"clerk"}},
	{Name: "mcp_listen", Label: "MCP listen address", Group: "Runtime", Type: "text", Kinds: []string{"clerk"}},
	{Name: "mcp_bearer_token", Label: "MCP bearer token", Group: "Runtime", Type: "text", Kinds: []string{"clerk"}},
	{Name: "max_response_bytes", Label: "Max response bytes", Group: "Runtime", Type: "int", Kinds: []string{"clerk", "direct"}},
	{Name: "invalid_attempt_limit", Label: "Invalid attempt limit", Group: "Runtime", Type: "int", Kinds: []string{"clerk", "direct"}},

	{Name: "input_prefix", Label: "S3 input prefix", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "output_prefix", Label: "S3 output prefix", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "output_root", Label: "S3 output root", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "exec_ami", Label: "Exec AMI", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "dev_host", Label: "Dev host", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "remote_attest_dir", Label: "Remote attest directory", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "aws_region", Label: "AWS region", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "instance_type", Label: "Instance type", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "iam_instance_profile", Label: "IAM instance profile", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "image_tar_s3", Label: "Image tar S3 URI", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "root_volume_size_gb", Label: "Root volume size GiB", Group: "Attestation", Type: "int", Kinds: []string{"attested"}, Attestation: true},
	{Name: "exec_poll_attempts", Label: "Exec poll attempts", Group: "Attestation", Type: "int", Kinds: []string{"attested"}, Attestation: true},
	{Name: "poll_interval_seconds", Label: "Poll interval seconds", Group: "Attestation", Type: "int", Kinds: []string{"attested"}, Attestation: true},
	{Name: "timeout_seconds", Label: "Driver timeout seconds", Group: "Attestation", Type: "int", Kinds: []string{"attested"}, Attestation: true},
	{Name: "expected_pcr4", Label: "Expected PCR4", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "expected_pcr7", Label: "Expected PCR7", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "expected_pcr12", Label: "Expected PCR12", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "driver_path", Label: "Driver path", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "uv", Label: "uv executable", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
	{Name: "parser", Label: "Attestation parser path", Group: "Attestation", Type: "text", Kinds: []string{"attested"}, Attestation: true},
}

// FieldGroup is one fieldset on the start form.
type FieldGroup struct {
	Name   string
	Fields []Field
}

var groupOrder = []string{"Case", "Council", "Lawyers", "Runtime", "Attestation"}

// GroupsFor returns the form groups that apply to one kind, in display
// order.
func GroupsFor(kind string) []FieldGroup {
	var groups []FieldGroup
	for _, g := range groupOrder {
		var fs []Field
		for _, f := range Fields {
			if f.Group == g && kindApplies(f, kind) {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			groups = append(groups, FieldGroup{Name: g, Fields: fs})
		}
	}
	return groups
}

func kindApplies(f Field, kind string) bool {
	for _, k := range f.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// BuildPayload converts submitted form values into the service create
// payload for one kind.  Blank fields are omitted.  It returns the
// payload and any field-level problems.
func BuildPayload(kind string, value func(string) string) (map[string]any, []string) {
	payload := map[string]any{}
	attestation := map[string]any{}
	var problems []string
	for _, f := range Fields {
		if !kindApplies(f, kind) {
			continue
		}
		raw := strings.TrimSpace(value(f.Name))
		if raw == "" {
			continue
		}
		var v any
		switch f.Type {
		case "int":
			n, err := strconv.Atoi(raw)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: %q is not an integer", f.Name, raw))
				continue
			}
			v = n
		case "list":
			var items []string
			for _, line := range strings.Split(raw, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					items = append(items, line)
				}
			}
			v = items
		default:
			v = raw
		}
		if f.Attestation {
			attestation[f.Name] = v
		} else {
			payload[f.Name] = v
		}
	}
	if kind == "attested" {
		payload["execution"] = map[string]any{"mode": "attested", "attestation": attestation}
	}
	return payload, problems
}
