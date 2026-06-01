package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"adjudication/common/acp"
)

const councilBackendPI = "pi"

func (rc *runContext) executeCouncilACPOpportunity(ctx context.Context, opportunity Opportunity, seat CouncilSeat) error {
	turn := rc.turn
	ctx, cancel := withTimeout(ctx, rc.cfg.Runtime.CouncilTimeout())
	defer cancel()

	session, err := rc.ensureCouncilACPSession(ctx, seat)
	if err != nil {
		return err
	}
	client := session.client

	transcript := make([]map[string]any, 0)
	var mu sync.Mutex
	appendTranscript := func(entry map[string]any) {
		mu.Lock()
		transcript = append(transcript, entry)
		mu.Unlock()
	}
	decisionSubmitted := false
	evidenceBudget := &evidenceReadBudget{}
	responseBytes := 0
	lastAgentToolStatus := map[string]string{}
	pendingAgentToolInput := map[string]any{}
	countedToolInput := map[string]bool{}
	var notifyErr error
	setNotifyErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if notifyErr != nil {
			return
		}
		notifyErr = err
		cancel()
	}
	accumulateResponseBytes := func(value any) {
		size, err := jsonPayloadSize(value)
		if err != nil {
			setNotifyErr(err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if notifyErr != nil {
			return
		}
		responseBytes += size
		if responseBytes > rc.cfg.Runtime.MaxResponseBytes {
			notifyErr = fmt.Errorf("council ACP response exceeded byte limit of %d", rc.cfg.Runtime.MaxResponseBytes)
			cancel()
		}
	}

	unsub := client.OnNotification(func(note acp.Notification) {
		if note.Method != "session/update" {
			return
		}
		update := mapAny(note.Params["update"])
		switch mapString(update["sessionUpdate"]) {
		case "agent_message_chunk", "agent_thought_chunk":
			content := mapAny(update["content"])
			text := mapString(content["text"])
			if text != "" {
				accumulateResponseBytes(text)
				appendTranscript(map[string]any{"assistant_text": text})
				_ = rc.recordEventAtTurn(turn, "council_agent_text", "council", opportunity.Phase, map[string]any{"member_id": seat.MemberID, "text": text})
			}
		case "tool_call":
			toolCallID := mapString(update["toolCallId"])
			rawInput := update["rawInput"]
			if toolCallID != "" && rawInput != nil && !countedToolInput[toolCallID] {
				accumulateResponseBytes(rawInput)
				countedToolInput[toolCallID] = true
			}
			entry := map[string]any{
				"member_id":    seat.MemberID,
				"tool_call_id": toolCallID,
				"title":        mapString(update["title"]),
				"status":       mapString(update["status"]),
				"raw_input":    rawInput,
			}
			if toolCallID != "" {
				lastAgentToolStatus[toolCallID] = mapString(update["status"])
			}
			appendTranscript(map[string]any{"agent_tool_call": entry})
			_ = rc.recordEventAtTurn(turn, "council_agent_tool_call", "council", opportunity.Phase, entry)
		case "tool_call_update":
			toolCallID := mapString(update["toolCallId"])
			status := mapString(update["status"])
			rawInput := update["rawInput"]
			rawOutput := update["rawOutput"]
			if toolCallID != "" && rawInput != nil && !countedToolInput[toolCallID] {
				accumulateResponseBytes(rawInput)
				countedToolInput[toolCallID] = true
			}
			if toolCallID != "" && status == "pending" && rawInput != nil && rawOutput == nil {
				pendingAgentToolInput[toolCallID] = rawInput
				if lastAgentToolStatus[toolCallID] == status {
					return
				}
				lastAgentToolStatus[toolCallID] = status
				return
			}
			entry := map[string]any{
				"member_id":    seat.MemberID,
				"tool_call_id": toolCallID,
				"status":       status,
			}
			if rawInput != nil {
				entry["raw_input"] = rawInput
			} else if buffered := pendingAgentToolInput[toolCallID]; buffered != nil {
				entry["raw_input"] = buffered
			}
			if rawOutput != nil {
				entry["raw_output"] = rawOutput
			}
			if toolCallID != "" && entry["raw_input"] == nil && entry["raw_output"] == nil && status != "" && lastAgentToolStatus[toolCallID] == status {
				return
			}
			if toolCallID != "" {
				if status != "" {
					lastAgentToolStatus[toolCallID] = status
				}
				if status != "pending" {
					delete(pendingAgentToolInput, toolCallID)
				}
			}
			appendTranscript(map[string]any{"agent_tool_update": entry})
			_ = rc.recordEventAtTurn(turn, "council_agent_tool_update", "council", opportunity.Phase, entry)
		}
	})
	defer unsub()

	client.HandleMethod(acpCustomMethod("get_case"), func(_ context.Context, _ map[string]any) (map[string]any, error) {
		view := rc.councilView(seat, opportunity)
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("get_case"), "result": view})
		return map[string]any{"text": marshalInline(view), "case": view}, nil
	})
	client.HandleMethod(acpCustomMethod("list_evidence"), func(_ context.Context, _ map[string]any) (map[string]any, error) {
		if !jurorEvidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("juror evidence access is not allowed in phase %q", opportunity.Phase)
		}
		evidence := rc.listVisibleEvidence()
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("list_evidence"), "member_id": seat.MemberID, "evidence_count": len(evidence)})
		return map[string]any{"text": marshalInline(map[string]any{"evidence": evidence}), "evidence": evidence}, nil
	})
	client.HandleMethod(acpCustomMethod("stat_evidence"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if !jurorEvidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("juror evidence access is not allowed in phase %q", opportunity.Phase)
		}
		evidence, err := rc.statEvidence(mapString(params["evidence_id"]))
		if err != nil {
			return nil, err
		}
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("stat_evidence"), "member_id": seat.MemberID, "evidence_id": evidence.EvidenceID})
		return map[string]any{
			"text":               marshalInline(map[string]any{"evidence": evidence}),
			"evidence":           evidence,
			"allowed_operations": []string{"read_range", "materialize"},
			"limits": map[string]any{
				"max_read_bytes":                       rc.cfg.Policy.MaxEvidenceReadBytes,
				"max_reads_per_opportunity":            rc.cfg.Policy.MaxEvidenceReadsPerOpportunity,
				"max_read_bytes_per_opportunity":       rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity,
				"remaining_read_bytes_for_opportunity": remainingCapacity(rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, evidenceBudget.bytes),
				"remaining_reads_for_opportunity":      remainingCapacity(rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, evidenceBudget.reads),
			},
		}, nil
	})
	client.HandleMethod(acpCustomMethod("read_evidence_range"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if !jurorEvidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("juror evidence access is not allowed in phase %q", opportunity.Phase)
		}
		offset, err := requiredIntParam(params, "offset")
		if err != nil {
			return nil, err
		}
		length, err := requiredIntParam(params, "length")
		if err != nil {
			return nil, err
		}
		result, err := rc.readEvidenceRange(mapString(params["evidence_id"]), int64(offset), length, evidenceBudget)
		if err != nil {
			return nil, err
		}
		result["remaining_read_bytes_for_opportunity"] = remainingCapacity(rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity, evidenceBudget.bytes)
		result["remaining_reads_for_opportunity"] = remainingCapacity(rc.cfg.Policy.MaxEvidenceReadsPerOpportunity, evidenceBudget.reads)
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("read_evidence_range"), "member_id": seat.MemberID, "evidence_id": result["evidence_id"], "offset": result["offset"], "length": result["length"]})
		if err := rc.recordEventAtTurn(turn, "evidence_read", "council", opportunity.Phase, map[string]any{
			"member_id":   seat.MemberID,
			"evidence_id": result["evidence_id"],
			"offset":      result["offset"],
			"length":      result["length"],
			"byte_count":  result["length"],
		}); err != nil {
			setNotifyErr(err)
		}
		return result, nil
	})
	client.HandleMethod(acpCustomMethod("materialize_evidence"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if !jurorEvidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("juror evidence access is not allowed in phase %q", opportunity.Phase)
		}
		result, err := rc.materializeEvidence(session.workspaceDir, mapString(params["evidence_id"]))
		if err != nil {
			return nil, err
		}
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("materialize_evidence"), "member_id": seat.MemberID, "evidence_id": result["evidence_id"], "workspace_path": result["workspace_path"]})
		if err := rc.recordEventAtTurn(turn, "evidence_materialized", "council", opportunity.Phase, map[string]any{
			"member_id":      seat.MemberID,
			"evidence_id":    result["evidence_id"],
			"workspace_path": result["workspace_path"],
			"byte_count":     result["size_bytes"],
		}); err != nil {
			setNotifyErr(err)
		}
		return result, nil
	})
	client.HandleMethod(acpCustomMethod("submit_council_vote"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if decisionSubmitted {
			return nil, fmt.Errorf("council vote already submitted for this opportunity")
		}
		payload := cloneMap(params)
		payload["member_id"] = seat.MemberID
		if mapString(payload["vote"]) == "" || mapString(payload["rationale"]) == "" {
			return nil, fmt.Errorf("submit_council_vote requires vote and rationale")
		}
		stepResp, err := rc.cfg.Engine.Step(rc.state, "submit_council_vote", "council", payload)
		if err != nil {
			return nil, err
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			return nil, fmt.Errorf("%s", mapString(stepResp["error"]))
		}
		rc.state = mapAny(stepResp["state"])
		if rc.lawyerAPI != nil {
			rc.lawyerAPI.signalChanged()
		}
		decisionSubmitted = true
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("submit_council_vote"), "member_id": seat.MemberID, "payload": payload, "step_result": stepResp})
		if err := rc.recordEventAtTurn(turn, "council_vote", "council", opportunity.Phase, map[string]any{
			"member_id": seat.MemberID,
			"model":     seat.Model,
			"backend":   councilBackendPI,
			"payload":   payload,
		}); err != nil {
			setNotifyErr(err)
		}
		return map[string]any{"text": "Council vote accepted."}, nil
	})

	sessionResp, err := client.NewSession(ctx, session.sessionPath)
	if err != nil {
		return err
	}
	prompt, err := rc.buildCouncilACPPrompt(seat, opportunity)
	if err != nil {
		return err
	}
	if _, err := client.Prompt(ctx, acp.PromptRequest{
		SessionID: sessionResp.SessionID,
		Prompt:    []acp.TextBlock{{Type: "text", Text: prompt}},
	}); err != nil {
		mu.Lock()
		defer mu.Unlock()
		if notifyErr != nil {
			return notifyErr
		}
		if decisionSubmitted {
			return nil
		}
		if isCouncilTimeoutError(err) {
			return rc.removeTimedOutCouncilMember(opportunity, seat, err)
		}
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if notifyErr != nil {
		return notifyErr
	}
	if !decisionSubmitted {
		return fmt.Errorf("council ACP agent %s did not submit a vote", seat.MemberID)
	}
	return nil
}

func (rc *runContext) ensureCouncilACPSession(ctx context.Context, seat CouncilSeat) (*acpPersistentSession, error) {
	key := councilACPKey(seat.MemberID)
	if session, ok := rc.acpSessions[key]; ok {
		return session, nil
	}
	command := strings.TrimSpace(rc.cfg.CouncilACPCommand)
	if command == "" {
		return nil, fmt.Errorf("council ACP command is required")
	}
	model := strings.TrimSpace(seat.Model)
	if model == "" {
		return nil, fmt.Errorf("council member %s has no model", seat.MemberID)
	}
	spec, err := parseAttorneyModel(model)
	if err != nil {
		return nil, fmt.Errorf("parse council model for %s: %w", seat.MemberID, err)
	}
	if spec.SearchRequested {
		return nil, fmt.Errorf("council PI backend does not allow web-search-enabled model %q for %s", model, seat.MemberID)
	}
	sessionCwd := strings.TrimSpace(rc.cfg.CouncilACPSessionCwd)
	if sessionCwd == "" {
		sessionCwd = rc.cfg.OutputDir
	}
	sessionACPPath := sessionCwd
	env := append([]string{}, rc.cfg.ACPEnv...)
	cleanup := func() error { return nil }
	workspaceDir := ""
	if usesPIContainerWrapper(command) {
		containerHomeDir, closeHome, err := prepareEphemeralPIHome(rc.cfg.CommonRoot, model, "")
		if err != nil {
			return nil, err
		}
		cleanup = closeHome
		env = append(env, "PI_CONTAINER_HOME_DIR="+containerHomeDir)
		sessionACPPath = defaultRemoteSessionCwd
		workspaceDir = containerHomeDir
	} else if sessionCwd == "" {
		cwd, err := filepath.Abs(rc.cfg.OutputDir)
		if err != nil {
			return nil, fmt.Errorf("resolve council ACP cwd: %w", err)
		}
		sessionCwd = cwd
		sessionACPPath = cwd
	}
	if workspaceDir == "" {
		workspaceDir = sessionCwd
		if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
			return nil, errors.Join(fmt.Errorf("create council workspace dir: %w", err), cleanup())
		}
	}
	env = append(env, "PI_ACP_CLIENT_TOOLS="+marshalInline(jurorACPClientToolSpecs(workspaceDir != "")))
	client, err := acp.NewClient(acp.Config{
		Command: command,
		Args:    rc.cfg.ACPArgs,
		Cwd:     sessionCwd,
		Env:     env,
	})
	if err != nil {
		return nil, errors.Join(err, cleanup())
	}
	session := &acpPersistentSession{
		client:       client,
		sessionPath:  sessionACPPath,
		workspaceDir: workspaceDir,
		cleanup:      cleanup,
	}
	if _, err := client.Initialize(ctx, 1); err != nil {
		return nil, errors.Join(err, session.Close())
	}
	rc.acpSessions[key] = session
	return session, nil
}

func councilACPKey(memberID string) string {
	return "council:" + strings.TrimSpace(memberID)
}

func jurorEvidenceAccessAllowed(opportunity Opportunity) bool {
	return opportunity.Role == "council" && opportunity.Phase == "deliberation"
}

func (rc *runContext) councilView(seat CouncilSeat, opportunity Opportunity) map[string]any {
	caseObj := mapAny(rc.state["case"])
	return map[string]any{
		"proposition":       rc.complaint.Proposition,
		"evidence_standard": currentEvidenceStandard(rc.state, rc.cfg.Policy),
		"phase":             currentPhase(rc.state),
		"opportunity": map[string]any{
			"id":        opportunity.ID,
			"role":      opportunity.Role,
			"phase":     opportunity.Phase,
			"member_id": seat.MemberID,
		},
		"record": map[string]any{
			"evidence":           rc.listVisibleEvidence(),
			"openings":           mapList(caseObj["openings"]),
			"arguments":          mapList(caseObj["arguments"]),
			"rebuttals":          mapList(caseObj["rebuttals"]),
			"surrebuttals":       mapList(caseObj["surrebuttals"]),
			"closings":           mapList(caseObj["closings"]),
			"submitted_evidence": mapList(caseObj["submitted_evidence"]),
			"exhibits":           rc.attorneyExhibits(),
			"technical_reports":  mapList(caseObj["technical_reports"]),
			"prior_votes":        mapList(caseObj["council_votes"]),
		},
		"limits": map[string]any{
			"max_evidence_read_bytes":                 rc.cfg.Policy.MaxEvidenceReadBytes,
			"max_evidence_reads_per_opportunity":      rc.cfg.Policy.MaxEvidenceReadsPerOpportunity,
			"max_evidence_read_bytes_per_opportunity": rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity,
			"max_response_bytes":                      rc.cfg.Runtime.MaxResponseBytes,
		},
		"council_member": map[string]any{
			"member_id": seat.MemberID,
			"model":     seat.Model,
		},
	}
}

func (rc *runContext) buildCouncilACPPrompt(seat CouncilSeat, opportunity Opportunity) (string, error) {
	base, err := rc.buildCouncilPrompt(seat, opportunity)
	if err != nil {
		return "", err
	}
	return base + "\n\nJuror agent instructions:\n" +
		"You are a juror. Your task is to decide the proposition from the admitted record.\n" +
		"You may examine admitted evidence through the read-only AAR tools. Use aar_list_evidence, aar_stat_evidence, aar_read_evidence_range, or aar_materialize_evidence when exact bytes, metadata, or exhibit contents matter.\n" +
		"Do not search the web, introduce new facts, create new evidence, upload evidence, or treat local workspace paths as record identities. Evidence identity is evidence_id plus SHA-256.\n" +
		"When ready, call aar_submit_council_vote exactly once with vote=demonstrated or vote=not_demonstrated and a concise rationale.\n", nil
}

func jurorACPClientToolSpecs(includeWorkspaceWriter bool) []map[string]any {
	specs := []map[string]any{
		getCaseToolSpec(),
		listEvidenceToolSpec(),
		statEvidenceToolSpec(),
		readEvidenceRangeToolSpec(),
	}
	if includeWorkspaceWriter {
		specs = append(specs, jurorMaterializeEvidenceToolSpec())
	}
	specs = append(specs, submitCouncilVoteToolSpec())
	return specs
}

func jurorMaterializeEvidenceToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("materialize_evidence"),
		"toolName":    "aar_materialize_evidence",
		"description": "Copy one visible evidence into the managed juror workspace and return its workspace path. The evidence_id and hash remain the record identity.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"evidence_id": map[string]any{"type": "string"},
			},
			"required":             []string{"evidence_id"},
			"additionalProperties": false,
		},
	}
}

func submitCouncilVoteToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("submit_council_vote"),
		"toolName":    "aar_submit_council_vote",
		"description": "Submit one council vote for the current deliberation opportunity.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"vote":      map[string]any{"type": "string", "enum": []string{"demonstrated", "not_demonstrated"}},
				"rationale": map[string]any{"type": "string"},
			},
			"required":             []string{"vote", "rationale"},
			"additionalProperties": false,
		},
	}
}
