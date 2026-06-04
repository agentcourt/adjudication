package runner

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"adjudication/common/acp"
)

type acpPersistentSession struct {
	client         *acp.Client
	sessionPath    string
	workspaceDir   string
	workProductDir string
	cleanup        func() error
}

func (s *acpPersistentSession) Close() error {
	if s == nil {
		return nil
	}
	var err error
	if s.client != nil {
		err = errors.Join(err, s.client.Close())
	}
	if s.cleanup != nil {
		err = errors.Join(err, s.cleanup())
	}
	return err
}

func (rc *runContext) executeAttorneyOpportunity(ctx context.Context, _ any, opportunity Opportunity) error {
	turn := rc.turn
	ctx, cancel := withTimeout(ctx, rc.cfg.Runtime.AttorneyACPTimeout())
	defer cancel()

	session, err := rc.ensureACPSession(ctx, opportunity.Role)
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
	invalidDecisionReasons := make([]string, 0)
	responseBytes := 0
	lastAgentToolStatus := map[string]string{}
	pendingAgentToolInput := map[string]any{}
	countedToolInput := map[string]bool{}
	evidenceBudget := &evidenceReadBudget{}
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
	recordInvalidDecision := func(err error) error {
		invalidDecisionReasons = append(invalidDecisionReasons, strings.TrimSpace(err.Error()))
		feedbackErr := formatAttorneyInvalidDecisionError(opportunity, rc.cfg.Policy, invalidDecisionReasons, rc.cfg.Runtime.InvalidAttemptLimit)
		if len(invalidDecisionReasons) >= rc.cfg.Runtime.InvalidAttemptLimit {
			setNotifyErr(feedbackErr)
		}
		return feedbackErr
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
			notifyErr = fmt.Errorf("attorney response exceeded byte limit of %d", rc.cfg.Runtime.MaxResponseBytes)
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
				_ = rc.recordEventAtTurn(turn, "assistant_text", opportunity.Role, opportunity.Phase, map[string]any{"text": text})
			}
		case "tool_call":
			toolCallID := mapString(update["toolCallId"])
			rawInput := update["rawInput"]
			if toolCallID != "" && rawInput != nil && !countedToolInput[toolCallID] {
				accumulateResponseBytes(rawInput)
				countedToolInput[toolCallID] = true
			}
			entry := map[string]any{
				"tool_call_id": toolCallID,
				"title":        mapString(update["title"]),
				"status":       mapString(update["status"]),
				"raw_input":    rawInput,
			}
			if toolCallID != "" {
				lastAgentToolStatus[toolCallID] = mapString(update["status"])
			}
			appendTranscript(map[string]any{"agent_tool_call": entry})
			_ = rc.recordEventAtTurn(turn, "agent_tool_call", opportunity.Role, opportunity.Phase, entry)
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
			if toolCallID != "" &&
				entry["raw_input"] == nil &&
				entry["raw_output"] == nil &&
				status != "" &&
				lastAgentToolStatus[toolCallID] == status {
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
			_ = rc.recordEventAtTurn(turn, "agent_tool_update", opportunity.Role, opportunity.Phase, entry)
		}
	})
	defer unsub()

	client.HandleMethod(acpCustomMethod("get_case"), func(_ context.Context, _ map[string]any) (map[string]any, error) {
		view := rc.attorneyView(opportunity)
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("get_case"), "result": view})
		return map[string]any{
			"text": marshalInline(view),
			"case": view,
		}, nil
	})
	client.HandleMethod(acpCustomMethod("list_evidence"), func(_ context.Context, _ map[string]any) (map[string]any, error) {
		if !evidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("evidence access is not allowed in phase %q", opportunity.Phase)
		}
		evidence := rc.listVisibleEvidence()
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("list_evidence"), "evidence_count": len(evidence)})
		return map[string]any{
			"text":     marshalInline(map[string]any{"evidence": evidence}),
			"evidence": evidence,
		}, nil
	})
	client.HandleMethod(acpCustomMethod("stat_evidence"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if !evidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("evidence access is not allowed in phase %q", opportunity.Phase)
		}
		evidence, err := rc.statEvidence(mapString(params["evidence_id"]))
		if err != nil {
			return nil, err
		}
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("stat_evidence"), "evidence_id": evidence.EvidenceID})
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
		if !evidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("evidence access is not allowed in phase %q", opportunity.Phase)
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
		appendTranscript(map[string]any{
			"custom_method": acpCustomMethod("read_evidence_range"),
			"evidence_id":   result["evidence_id"],
			"offset":        result["offset"],
			"length":        result["length"],
		})
		if err := rc.recordEventAtTurn(turn, "evidence_read", opportunity.Role, opportunity.Phase, map[string]any{
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
		if !evidenceAccessAllowed(opportunity) {
			return nil, fmt.Errorf("evidence access is not allowed in phase %q", opportunity.Phase)
		}
		result, err := rc.materializeEvidence(session.workspaceDir, mapString(params["evidence_id"]))
		if err != nil {
			return nil, err
		}
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("materialize_evidence"), "evidence_id": result["evidence_id"], "workspace_path": result["workspace_path"]})
		if err := rc.recordEventAtTurn(turn, "evidence_materialized", opportunity.Role, opportunity.Phase, map[string]any{
			"evidence_id":    result["evidence_id"],
			"workspace_path": result["workspace_path"],
			"byte_count":     result["size_bytes"],
		}); err != nil {
			setNotifyErr(err)
		}
		return result, nil
	})
	client.HandleMethod(acpCustomMethod("begin_evidence_upload"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		session, err := rc.beginEvidenceUpload(opportunity, params)
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("begin_evidence_upload"), "upload_id": session.UploadID, "expected_size_bytes": session.ExpectedSizeBytes})
		return map[string]any{
			"upload_id":              session.UploadID,
			"max_chunk_bytes":        rc.cfg.Policy.MaxEvidenceChunkBytes,
			"remaining_upload_bytes": session.ExpectedSizeBytes,
		}, nil
	})
	client.HandleMethod(acpCustomMethod("write_evidence_chunk"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		offset, err := requiredIntParam(params, "offset")
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		session, n, err := rc.writeEvidenceChunk(mapString(params["upload_id"]), offset, mapString(params["content_base64"]))
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("write_evidence_chunk"), "upload_id": session.UploadID, "offset": session.ReceivedBytes - n, "length": n})
		return map[string]any{
			"upload_id":              session.UploadID,
			"accepted_offset":        session.ReceivedBytes - n,
			"accepted_length":        n,
			"received_bytes":         session.ReceivedBytes,
			"remaining_upload_bytes": remainingCapacity(session.ExpectedSizeBytes, session.ReceivedBytes),
		}, nil
	})
	client.HandleMethod(acpCustomMethod("commit_evidence_upload"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		uploadID := mapString(params["upload_id"])
		session := rc.uploadSessions[uploadID]
		if session == nil {
			return nil, recordInvalidDecision(fmt.Errorf("unknown upload_id %q", uploadID))
		}
		if expected := strings.ToLower(mapString(params["expected_sha256"])); expected != "" {
			session.ExpectedSHA256 = expected
		}
		meta, err := rc.prepareEvidenceUploadCommit(session, mapString(params["preferred_filename_ext"]))
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		payload := submittedEvidencePayload(meta)
		stepResp, err := rc.cfg.Engine.Step(rc.state, "submit_evidence", opportunity.Role, payload)
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			return nil, recordInvalidDecision(fmt.Errorf("%s", mapString(stepResp["error"])))
		}
		meta, file, evidence, err := rc.finalizeEvidenceUpload(session, meta)
		if err != nil {
			return nil, err
		}
		rc.state = mapAny(stepResp["state"])
		rc.caseFiles = append(rc.caseFiles, file)
		rc.fileByID[file.EvidenceID] = file
		rc.submittedEvidence = append(rc.submittedEvidence, meta)
		appendTranscript(map[string]any{"custom_method": acpCustomMethod("commit_evidence_upload"), "upload_id": uploadID, "evidence_meta": meta, "evidence": evidence})
		if err := rc.recordEventAtTurn(turn, "submitted_evidence", opportunity.Role, opportunity.Phase, map[string]any{
			"evidence_id":         meta.EvidenceID,
			"title":               meta.Title,
			"source_url":          meta.SourceURL,
			"source_description":  meta.SourceDescription,
			"mime_type":           meta.MimeType,
			"retrieval_timestamp": meta.RetrievalTimestamp,
			"relevance":           meta.Relevance,
			"sha256":              meta.SHA256,
			"size_bytes":          meta.SizeBytes,
		}); err != nil {
			setNotifyErr(err)
		}
		return map[string]any{
			"text":        fmt.Sprintf("Evidence upload accepted as evidence_id %s. Cite this evidence_id in offered_evidence if you want it admitted as an exhibit.", meta.EvidenceID),
			"evidence_id": meta.EvidenceID,
			"evidence":    evidence,
		}, nil
	})
	client.HandleMethod(acpCustomMethod("submit_evidence"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		meta, raw, err := rc.prepareSubmittedEvidence(opportunity, params)
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		payload := submittedEvidencePayload(meta)
		stepResp, err := rc.cfg.Engine.Step(rc.state, "submit_evidence", opportunity.Role, payload)
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			return nil, recordInvalidDecision(fmt.Errorf("%s", mapString(stepResp["error"])))
		}
		file, err := rc.writeSubmittedEvidenceFile(meta, raw)
		if err != nil {
			return nil, err
		}
		evidence, err := rc.registerSubmittedEvidenceEvidence(meta, file)
		if err != nil {
			return nil, err
		}
		meta.EvidenceID = evidence.EvidenceID
		rc.state = mapAny(stepResp["state"])
		rc.caseFiles = append(rc.caseFiles, file)
		rc.fileByID[file.EvidenceID] = file
		rc.submittedEvidence = append(rc.submittedEvidence, meta)
		appendTranscript(map[string]any{
			"custom_method": acpCustomMethod("submit_evidence"),
			"result":        meta,
		})
		if err := rc.recordEventAtTurn(turn, "submitted_evidence", opportunity.Role, opportunity.Phase, map[string]any{
			"evidence_id":         meta.EvidenceID,
			"title":               meta.Title,
			"source_url":          meta.SourceURL,
			"source_description":  meta.SourceDescription,
			"mime_type":           meta.MimeType,
			"retrieval_timestamp": meta.RetrievalTimestamp,
			"relevance":           meta.Relevance,
			"sha256":              meta.SHA256,
			"size_bytes":          meta.SizeBytes,
		}); err != nil {
			setNotifyErr(err)
		}
		return map[string]any{
			"text":        fmt.Sprintf("Evidence accepted as evidence_id %s. Cite this evidence_id in offered_evidence if you want it admitted as an exhibit.", meta.EvidenceID),
			"evidence_id": meta.EvidenceID,
			"evidence":    evidence,
		}, nil
	})
	client.HandleMethod(acpCustomMethod("submit_decision"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if decisionSubmitted {
			return nil, fmt.Errorf("decision already submitted for this opportunity")
		}
		actionType, payload, err := attorneyDecision(opportunity, params, rc.fileByID, rc.cfg.Policy)
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		if err := rc.validateAttorneyPayloadAgainstState(opportunity, actionType, payload); err != nil {
			return nil, recordInvalidDecision(err)
		}
		stepResp, err := rc.cfg.Engine.Step(rc.state, actionType, opportunity.Role, payload)
		if err != nil {
			return nil, recordInvalidDecision(err)
		}
		if ok, _ := stepResp["ok"].(bool); !ok {
			return nil, recordInvalidDecision(fmt.Errorf("%s", mapString(stepResp["error"])))
		}
		rc.state = mapAny(stepResp["state"])
		decisionSubmitted = true
		appendTranscript(map[string]any{
			"decision":    params,
			"action":      actionType,
			"payload":     payload,
			"step_result": stepResp,
		})
		if err := rc.recordEventAtTurn(turn, "attorney_action", opportunity.Role, opportunity.Phase, map[string]any{
			"opportunity_id": opportunity.ID,
			"action_type":    actionType,
			"payload":        payload,
		}); err != nil {
			setNotifyErr(err)
		}
		return map[string]any{
			"text": "Decision accepted.",
		}, nil
	})

	sessionResp, err := client.NewSession(ctx, session.sessionPath)
	if err != nil {
		return err
	}
	prompt, err := rc.buildAttorneyPrompt(opportunity)
	if err != nil {
		return err
	}
	if _, err := client.Prompt(ctx, acp.PromptRequest{
		SessionID: sessionResp.SessionID,
		Prompt:    []acp.TextBlock{{Type: "text", Text: prompt}},
		Meta:      attorneyPromptMeta(opportunity, session.workspaceDir != ""),
	}); err != nil {
		mu.Lock()
		defer mu.Unlock()
		if notifyErr != nil {
			return notifyErr
		}
		if decisionSubmitted {
			return nil
		}
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if notifyErr != nil {
		return notifyErr
	}
	if !decisionSubmitted {
		return fmt.Errorf("acp attorney did not submit a decision")
	}
	return nil
}

func (rc *runContext) ensureACPSession(ctx context.Context, role string) (*acpPersistentSession, error) {
	if session, ok := rc.acpSessions[role]; ok {
		return session, nil
	}
	attorney, err := rc.attorneyInfo(role)
	if err != nil {
		return nil, err
	}
	sessionCwd := attorney.SessionCwd
	sessionACPPath := sessionCwd
	env := append([]string{}, rc.cfg.ACPEnv...)
	cleanup := func() error { return nil }
	workspaceDir := ""
	workProductDir := ""
	instructionsPath := strings.TrimSpace(rc.cfg.AttorneyInstructionsPath)
	if usesPIContainerWrapper(attorney.ACPCommand) {
		containerHomeDir, closeHome, err := prepareEphemeralPIHome(rc.cfg.CommonRoot, attorney.Model, instructionsPath)
		if err != nil {
			return nil, err
		}
		cleanup = closeHome
		env = append(env, "PI_CONTAINER_HOME_DIR="+containerHomeDir)
		if instructionsPath != "" {
			env = append(env, "PI_ACP_INSTRUCTIONS_FILE="+stagedAttorneyInstructionsACPPath)
		}
		sessionACPPath = "/home/user"
		workspaceDir = containerHomeDir
		workProductDir = filepath.Join(containerHomeDir, "work-product")
		if err := os.MkdirAll(workProductDir, 0o755); err != nil {
			return nil, errors.Join(fmt.Errorf("create work-product dir: %w", err), cleanup())
		}
	} else if attorney.ACPTransport == "stdio" && instructionsPath != "" {
		env = append(env, "PI_ACP_INSTRUCTIONS_FILE="+instructionsPath)
	}
	env = append(env, "PI_ACP_CLIENT_TOOLS="+marshalInline(acpClientToolSpecs(workspaceDir != "")))
	client, err := acp.NewClient(acp.Config{
		Command:  attorney.ACPCommand,
		Endpoint: attorney.ACPEndpoint,
		Args:     rc.cfg.ACPArgs,
		Cwd:      sessionCwd,
		Env:      env,
	})
	if err != nil {
		return nil, errors.Join(err, cleanup())
	}
	session := &acpPersistentSession{
		client:         client,
		sessionPath:    sessionACPPath,
		workspaceDir:   workspaceDir,
		workProductDir: workProductDir,
		cleanup:        cleanup,
	}
	if _, err := client.Initialize(ctx, 1); err != nil {
		return nil, errors.Join(err, session.Close())
	}
	rc.acpSessions[role] = session
	if strings.TrimSpace(workProductDir) != "" {
		rc.workProductDirs[role] = workProductDir
	}
	return session, nil
}

func (rc *runContext) closeACPSessions() error {
	if len(rc.acpSessions) == 0 {
		return nil
	}
	roleNames := make([]string, 0, len(rc.acpSessions))
	for role := range rc.acpSessions {
		roleNames = append(roleNames, role)
	}
	sort.Strings(roleNames)
	var err error
	for _, role := range roleNames {
		session := rc.acpSessions[role]
		delete(rc.acpSessions, role)
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ACP session role=%s: %w", role, closeErr))
		}
	}
	return err
}

func attorneyDecision(opportunity Opportunity, params map[string]any, fileByID map[string]CaseFile, policy Policy) (string, map[string]any, error) {
	kind := mapString(params["kind"])
	switch kind {
	case "pass":
		if !opportunity.MayPass {
			return "", nil, fmt.Errorf("passing is not allowed in this opportunity")
		}
		switch opportunity.Phase {
		case "rebuttals", "surrebuttals":
			return "pass_phase_opportunity", map[string]any{}, nil
		default:
			return "", nil, fmt.Errorf("passing is not allowed in phase %q", opportunity.Phase)
		}
	case "tool":
		toolName := mapString(params["tool_name"])
		if !slices.Contains(opportunity.AllowedTools, toolName) {
			return "", nil, fmt.Errorf("tool %q is not allowed in this opportunity", toolName)
		}
		payload := normalizePayload(params["payload"])
		if err := validateAttorneyPayload(toolName, payload, fileByID, policy); err != nil {
			return "", nil, err
		}
		return toolName, payload, nil
	default:
		return "", nil, fmt.Errorf("submit_decision kind must be tool or pass")
	}
}

func validateAttorneyPayload(actionType string, payload map[string]any, fileByID map[string]CaseFile, policy Policy) error {
	switch actionType {
	case "record_opening_statement":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
	case "deliver_closing_statement":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if len(listOfMaps(payload["offered_evidence"])) != 0 {
			return fmt.Errorf("offered_evidence are allowed only in arguments and rebuttals")
		}
		if len(listOfMaps(payload["technical_reports"])) != 0 {
			return fmt.Errorf("technical_reports are allowed only in arguments and rebuttals")
		}
	case "submit_argument":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if err := validateOfferedEvidence(payload["offered_evidence"], fileByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload["technical_reports"], policy); err != nil {
			return err
		}
	case "submit_rebuttal":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if err := validateOfferedEvidence(payload["offered_evidence"], fileByID, policy); err != nil {
			return err
		}
		if err := validateReports(payload["technical_reports"], policy); err != nil {
			return err
		}
	case "submit_surrebuttal":
		if mapString(payload["text"]) == "" {
			return fmt.Errorf("payload.text is required")
		}
		if len(listOfMaps(payload["offered_evidence"])) != 0 {
			return fmt.Errorf("offered_evidence are allowed only in arguments and rebuttals")
		}
		if len(listOfMaps(payload["technical_reports"])) != 0 {
			return fmt.Errorf("technical_reports are allowed only in arguments and rebuttals")
		}
	case "pass_phase_opportunity":
	default:
		return fmt.Errorf("unsupported action type %q", actionType)
	}
	return nil
}

func validateOfferedEvidence(value any, fileByID map[string]CaseFile, policy Policy) error {
	entries := listOfMaps(value)
	if len(entries) > policy.MaxExhibitsPerFiling {
		return fmt.Errorf("offered_evidence exceed per-filing limit of %d (attempted %d)", policy.MaxExhibitsPerFiling, len(entries))
	}
	for _, entry := range entries {
		evidenceID := mapString(entry["evidence_id"])
		if evidenceID == "" {
			return fmt.Errorf("offered_evidence entry requires evidence_id")
		}
		file, ok := fileByID[evidenceID]
		if !ok {
			return fmt.Errorf("unknown offered file %q; offered_evidence must use visible case evidence_id values, not workspace paths or downloaded filenames", evidenceID)
		}
		if file.SizeBytes > policy.MaxExhibitBytes {
			return fmt.Errorf("offered file %q exceeds byte limit of %d", evidenceID, policy.MaxExhibitBytes)
		}
	}
	return nil
}

func validateReports(value any, policy Policy) error {
	entries := listOfMaps(value)
	if len(entries) > policy.MaxReportsPerFiling {
		return fmt.Errorf("technical_reports exceed per-filing limit of %d (attempted %d)", policy.MaxReportsPerFiling, len(entries))
	}
	for _, entry := range entries {
		title := mapString(entry["title"])
		summary := mapString(entry["summary"])
		if title == "" {
			return fmt.Errorf("technical_reports entry requires title")
		}
		if summary == "" {
			return fmt.Errorf("technical_reports entry requires summary")
		}
		if len([]byte(title)) > policy.MaxReportTitleBytes {
			return fmt.Errorf("technical_reports title exceeds byte limit of %d", policy.MaxReportTitleBytes)
		}
		if len([]byte(summary)) > policy.MaxReportSummaryBytes {
			return fmt.Errorf("technical_reports summary exceeds byte limit of %d", policy.MaxReportSummaryBytes)
		}
	}
	return nil
}

func normalizePayload(value any) map[string]any {
	payload := mapAny(value)
	if len(payload) == 0 {
		return map[string]any{}
	}
	return cloneMap(payload)
}

func jsonPayloadSize(value any) (int, error) {
	wire, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("marshal response payload size: %w", err)
	}
	return len(wire), nil
}

func listOfMaps(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, raw := range v {
			entry, _ := raw.(map[string]any)
			if entry != nil {
				out = append(out, entry)
			}
		}
		return out
	default:
		return nil
	}
}

func ACPClientToolSpecs(includeWorkspaceWriter bool) []map[string]any {
	return acpClientToolSpecs(includeWorkspaceWriter)
}

func acpClientToolSpecs(includeWorkspaceWriter bool) []map[string]any {
	specs := []map[string]any{
		getCaseToolSpec(),
		listEvidenceToolSpec(),
		statEvidenceToolSpec(),
		readEvidenceRangeToolSpec(),
	}
	if includeWorkspaceWriter {
		specs = append(specs, materializeEvidenceToolSpec())
	}
	specs = append(specs, beginEvidenceUploadToolSpec(), writeEvidenceChunkToolSpec(), commitEvidenceUploadToolSpec(), submitEvidenceToolSpec(), submitDecisionToolSpec(nil))
	return specs
}

func getCaseToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("get_case"),
		"toolName":    "aar_get_case",
		"description": "Return the current visible arbitration record.",
		"parameters":  map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
	}
}

func listEvidenceToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("list_evidence"),
		"toolName":    "aar_list_evidence",
		"description": "List visible immutable record evidence with metadata, including case-packet files and accepted submitted evidence.",
		"parameters":  map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
	}
}

func statEvidenceToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("stat_evidence"),
		"toolName":    "aar_stat_evidence",
		"description": "Return metadata, allowed operations, and read limits for one visible evidence.",
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

func readEvidenceRangeToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("read_evidence_range"),
		"toolName":    "aar_read_evidence_range",
		"description": "Read a bounded byte range from one visible evidence as base64. This never mutates the record.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"evidence_id": map[string]any{"type": "string"},
				"offset":      map[string]any{"type": "integer", "minimum": 0},
				"length":      map[string]any{"type": "integer", "minimum": 1},
			},
			"required":             []string{"evidence_id", "offset", "length"},
			"additionalProperties": false,
		},
	}
}

func materializeEvidenceToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("materialize_evidence"),
		"toolName":    "aar_materialize_evidence",
		"description": "Copy one visible evidence into the managed attorney workspace and return its workspace path. The evidence_id and hash remain the record identity.",
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

func beginEvidenceUploadToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("begin_evidence_upload"),
		"toolName":    "aar_begin_evidence_upload",
		"description": "Begin a chunked upload session for source evidence or a derived evidence. Nothing is admitted until aar_commit_evidence_upload succeeds.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":               map[string]any{"type": "string"},
				"mime_type":           map[string]any{"type": "string"},
				"expected_size_bytes": map[string]any{"type": "integer", "minimum": 1},
				"expected_sha256":     map[string]any{"type": "string"},
				"source_url":          map[string]any{"type": "string"},
				"source_description":  map[string]any{"type": "string"},
				"retrieval_timestamp": map[string]any{"type": "string"},
				"relevance":           map[string]any{"type": "string"},
				"parent_evidence_id":  map[string]any{"type": "string"},
				"derivation_method":   map[string]any{"type": "string"},
			},
			"required":             []string{"title", "mime_type", "expected_size_bytes", "relevance"},
			"additionalProperties": false,
		},
	}
}

func writeEvidenceChunkToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("write_evidence_chunk"),
		"toolName":    "aar_write_evidence_chunk",
		"description": "Write one base64 chunk into an upload session at the next expected offset.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"upload_id":      map[string]any{"type": "string"},
				"offset":         map[string]any{"type": "integer", "minimum": 0},
				"content_base64": map[string]any{"type": "string"},
			},
			"required":             []string{"upload_id", "offset", "content_base64"},
			"additionalProperties": false,
		},
	}
}

func commitEvidenceUploadToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("commit_evidence_upload"),
		"toolName":    "aar_commit_evidence_upload",
		"description": "Verify and admit a completed evidence upload. The response returns evidence_id for later offered_evidence citations.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"upload_id":              map[string]any{"type": "string"},
				"expected_sha256":        map[string]any{"type": "string"},
				"preferred_filename_ext": map[string]any{"type": "string"},
			},
			"required":             []string{"upload_id"},
			"additionalProperties": false,
		},
	}
}

func submitEvidenceToolSpec() map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("submit_evidence"),
		"toolName":    "aar_submit_evidence",
		"description": "Submit source evidence with provenance. The accepted response returns an evidence_id that can be cited later in offered_evidence.",
		"parameters":  submittedEvidenceSchema(),
	}
}

func submitDecisionToolSpec(allowedTools []string) map[string]any {
	return map[string]any{
		"method":      acpCustomMethod("submit_decision"),
		"toolName":    "aar_submit_decision",
		"description": "Submit the legal act for the current opportunity.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind": map[string]any{"type": "string", "enum": []string{"tool", "pass"}},
				"tool_name": map[string]any{
					"type": "string",
					"enum": decisionToolEnum(allowedTools),
				},
				"payload": attorneyPayloadSchema(),
			},
			"required":             []string{"kind"},
			"additionalProperties": false,
		},
	}
}

func decisionToolEnum(allowedTools []string) []string {
	fallback := []string{
		"record_opening_statement",
		"submit_argument",
		"submit_rebuttal",
		"submit_surrebuttal",
		"deliver_closing_statement",
		"pass_phase_opportunity",
	}
	if len(allowedTools) == 0 {
		return fallback
	}
	out := make([]string, 0, len(allowedTools))
	for _, tool := range allowedTools {
		tool = strings.TrimSpace(tool)
		if tool != "" && !slices.Contains(out, tool) {
			out = append(out, tool)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func attorneyPayloadSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":              map[string]any{"type": "string"},
			"offered_evidence":  offeredEvidenceSchema(),
			"technical_reports": technicalReportsSchema(),
		},
		"additionalProperties": false,
	}
}

func offeredEvidenceSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"evidence_id": map[string]any{"type": "string"},
				"label":       map[string]any{"type": "string"},
			},
			"required":             []string{"evidence_id", "label"},
			"additionalProperties": false,
		},
	}
}

func technicalReportsSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":   map[string]any{"type": "string"},
				"summary": map[string]any{"type": "string"},
			},
			"required":             []string{"title", "summary"},
			"additionalProperties": false,
		},
	}
}

func submittedEvidenceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":                  map[string]any{"type": "string"},
			"source_url":             map[string]any{"type": "string"},
			"source_description":     map[string]any{"type": "string"},
			"retrieval_timestamp":    map[string]any{"type": "string"},
			"mime_type":              map[string]any{"type": "string"},
			"relevance":              map[string]any{"type": "string"},
			"content":                map[string]any{"type": "string"},
			"content_base64":         map[string]any{"type": "string"},
			"preferred_filename_ext": map[string]any{"type": "string"},
		},
		"required":             []string{"title", "mime_type", "relevance"},
		"additionalProperties": false,
	}
}

func acpToolSpecs(opportunity Opportunity, includeWorkspaceWriter bool) []map[string]any {
	specs := []map[string]any{getCaseToolSpec()}
	if evidenceAccessAllowed(opportunity) {
		specs = append(specs, listEvidenceToolSpec(), statEvidenceToolSpec(), readEvidenceRangeToolSpec())
		if includeWorkspaceWriter {
			specs = append(specs, materializeEvidenceToolSpec())
		}
		specs = append(specs, beginEvidenceUploadToolSpec(), writeEvidenceChunkToolSpec(), commitEvidenceUploadToolSpec(), submitEvidenceToolSpec())
	}
	specs = append(specs, submitDecisionToolSpec(opportunity.AllowedTools))
	return specs
}

func attorneyPromptMeta(opportunity Opportunity, includeWorkspaceWriter bool) map[string]any {
	return map[string]any{
		"clientTools": acpToolSpecs(opportunity, includeWorkspaceWriter),
	}
}

func evidenceAccessAllowed(opportunity Opportunity) bool {
	return opportunity.Phase == "arguments" || opportunity.Phase == "rebuttals"
}

func acpCustomMethod(name string) string {
	return "_aar/" + strings.TrimSpace(name)
}

func (rc *runContext) attorneyInfo(role string) (AttorneyRunInfo, error) {
	if attorney, ok := rc.attorneys[role]; ok {
		return attorney, nil
	}
	attorney, err := resolveAttorney(role, rc.cfg, rc.cfg.ComplaintPath)
	if err == nil {
		return attorney, nil
	}
	if strings.Contains(err.Error(), "ACP command is required") {
		model := strings.TrimSpace(rc.cfg.AttorneyModel)
		if model == "" {
			model = DefaultAttorneyModel
		}
		spec, parseErr := parseAttorneyModel(model)
		if parseErr != nil {
			return AttorneyRunInfo{}, parseErr
		}
		searchEnabled := spec.SearchRequested
		return AttorneyRunInfo{
			Role:          role,
			Model:         model,
			SearchEnabled: &searchEnabled,
			ACPTransport:  "stdio",
		}, nil
	}
	return AttorneyRunInfo{}, err
}

func (rc *runContext) attorneyView(opportunity Opportunity) map[string]any {
	limits := rc.attorneyLimits(opportunity)
	attorneyModel := ""
	if attorney, err := rc.attorneyInfo(opportunity.Role); err == nil {
		attorneyModel = attorney.Model
	}
	return map[string]any{
		"question":          rc.complaint.Question,
		"judgment_standard": currentJudgmentStandard(rc.state, rc.cfg.Policy),
		"attorney_model":    attorneyModel,
		"phase":             currentPhase(rc.state),
		"opportunity": map[string]any{
			"id":            opportunity.ID,
			"role":          opportunity.Role,
			"phase":         opportunity.Phase,
			"objective":     opportunity.Objective,
			"allowed_tools": opportunity.AllowedTools,
			"may_pass":      opportunity.MayPass,
		},
		"record": map[string]any{
			"evidence":           rc.listVisibleEvidence(),
			"openings":           mapList(mapAny(rc.state["case"])["openings"]),
			"arguments":          mapList(mapAny(rc.state["case"])["arguments"]),
			"rebuttals":          mapList(mapAny(rc.state["case"])["rebuttals"]),
			"surrebuttals":       mapList(mapAny(rc.state["case"])["surrebuttals"]),
			"closings":           mapList(mapAny(rc.state["case"])["closings"]),
			"submitted_evidence": mapList(mapAny(rc.state["case"])["submitted_evidence"]),
			"exhibits":           rc.attorneyExhibits(),
			"technical_reports":  mapList(mapAny(rc.state["case"])["technical_reports"]),
		},
		"limits":  limits,
		"council": rc.council,
	}
}

func (rc *runContext) attorneyCapabilitySection(role string) (string, error) {
	attorney, err := rc.attorneyInfo(role)
	if err != nil {
		return "", err
	}
	if attorney.SearchEnabled == nil {
		return "Attorney capabilities for this run:\nAARD is connected to a remote ACP attorney endpoint. Model selection and native tool availability are owned by that ACP attorney. Use the capabilities available in that environment and file record material through the AARD tools.", nil
	}
	if *attorney.SearchEnabled {
		return "Model capabilities for this run:\nNative web search through the model is available.", nil
	}
	return "Model capabilities for this run:\nNative web search through the model is not available.", nil
}

func (rc *runContext) buildAttorneyPrompt(opportunity Opportunity) (string, error) {
	view := rc.attorneyView(opportunity)
	visibleFilesSection := ""
	workspaceSection := ""
	workProductSection := ""
	if opportunity.Phase == "arguments" || opportunity.Phase == "rebuttals" {
		visibleFilesSection = "Visible evidence:\n" + marshalIndented(rc.listVisibleEvidence()) + "\n"
		workspaceSection = "If local tools need exact bytes, materialize the evidence into the workspace first. Do not reconstruct byte-sensitive evidence by hand. Use evidence_id plus hash as record identity; local paths are only workspace implementation details.\n"
	}
	if attorney, err := rc.attorneyInfo(opportunity.Role); err == nil && usesPIContainerWrapper(attorney.ACPCommand) {
		workProductSection = "Private work product: Use `/home/user/work-product/` for internal notes, timelines, source leads, adverse facts, unresolved questions, and draft analyses. This directory is not part of the record unless you later turn material from it into an exhibit or technical report. Its contents may be exported after the proceeding for review.\n"
	}
	capabilitySection, err := rc.attorneyCapabilitySection(opportunity.Role)
	if err != nil {
		return "", err
	}
	common, err := renderPromptFile("attorney-common.md", map[string]string{
		"ROLE":                       opportunity.Role,
		"PHASE":                      opportunity.Phase,
		"OBJECTIVE":                  opportunity.Objective,
		"QUESTION":                   rc.complaint.Question,
		"JUDGMENT_STANDARD":          currentJudgmentStandard(rc.state, rc.cfg.Policy),
		"MODEL_CAPABILITIES_SECTION": capabilitySection,
		"CURRENT_RECORD":             marshalIndented(view["record"]),
		"LIMITS_SECTION":             rc.attorneyLimitsSection(opportunity),
		"COUNCIL":                    marshalIndented(view["council"]),
		"VISIBLE_CASE_FILES_SECTION": visibleFilesSection,
		"WORKSPACE_SECTION":          workspaceSection,
		"WORK_PRODUCT_SECTION":       workProductSection,
		"ALLOWED_TOOLS":              strings.Join(opportunity.AllowedTools, ", "),
	})
	if err != nil {
		return "", err
	}
	phaseFile, err := attorneyPromptFile(opportunity.Phase)
	if err != nil {
		return "", err
	}
	phaseText, err := renderPromptFile(phaseFile, nil)
	if err != nil {
		return "", err
	}
	return common + "\n\n" + phaseText + "\n\nWhen you have submitted the legal act for this opportunity, reply exactly: decision-submitted.", nil
}

func (rc *runContext) prepareSubmittedEvidence(opportunity Opportunity, params map[string]any) (SubmittedEvidenceMeta, []byte, error) {
	if opportunity.Phase != "arguments" && opportunity.Phase != "rebuttals" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence is allowed only in arguments and rebuttals")
	}
	title := mapString(params["title"])
	mimeType := mapString(params["mime_type"])
	relevance := mapString(params["relevance"])
	sourceURL := mapString(params["source_url"])
	sourceDescription := mapString(params["source_description"])
	retrievalTimestamp := mapString(params["retrieval_timestamp"])
	if title == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires title")
	}
	if sourceURL == "" && sourceDescription == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires source_url or source_description")
	}
	if mimeType == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires mime_type")
	}
	if relevance == "" {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence requires relevance")
	}
	raw, err := submittedEvidenceContent(params)
	if err != nil {
		return SubmittedEvidenceMeta{}, nil, err
	}
	if len(raw) == 0 {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence content must not be empty")
	}
	if len(raw) > rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("direct submitted evidence exceeds byte limit of %d", rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes)
	}
	if len(raw) > rc.cfg.Policy.MaxSubmittedEvidenceBytes {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted evidence exceeds byte limit of %d", rc.cfg.Policy.MaxSubmittedEvidenceBytes)
	}
	if submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedEvidencePerSide {
		return SubmittedEvidenceMeta{}, nil, fmt.Errorf("submitted_evidence for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedEvidencePerSide)
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	name := submittedEvidenceFilename(len(rc.submittedEvidence)+1, opportunity.Role, sha, mimeType, mapString(params["preferred_filename_ext"]))
	return SubmittedEvidenceMeta{
		Phase:              opportunity.Phase,
		Role:               opportunity.Role,
		EvidenceID:         evidenceIDForFile(sha, name),
		Name:               name,
		Title:              title,
		SourceURL:          sourceURL,
		SourceDescription:  sourceDescription,
		MimeType:           mimeType,
		RetrievalTimestamp: retrievalTimestamp,
		Relevance:          relevance,
		SHA256:             sha,
		SizeBytes:          len(raw),
	}, raw, nil
}

func submittedEvidenceContent(params map[string]any) ([]byte, error) {
	content, hasContent := rawStringParam(params, "content")
	contentBase64 := mapString(params["content_base64"])
	if hasContent && contentBase64 != "" {
		return nil, fmt.Errorf("use content or content_base64, not both")
	}
	if contentBase64 != "" {
		raw, err := base64.StdEncoding.DecodeString(contentBase64)
		if err != nil {
			return nil, fmt.Errorf("decode content_base64: %w", err)
		}
		return raw, nil
	}
	if !hasContent {
		return nil, fmt.Errorf("submitted evidence requires content or content_base64")
	}
	return []byte(content), nil
}

func rawStringParam(params map[string]any, key string) (string, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return "", false
	}
	s, ok := value.(string)
	return s, ok
}

func submittedEvidenceFilename(index int, role string, sha string, mimeType string, preferredExt string) string {
	ext := sanitizeEvidenceExtension(preferredExt)
	if ext == "" {
		ext = evidenceExtensionForMIME(mimeType)
	}
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("submitted-evidence-%02d-%s-%s%s", index, sanitizeEvidenceComponent(role), sha[:12], ext)
}

func sanitizeEvidenceComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

func sanitizeEvidenceExtension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	if filepath.Base("x"+value) != "x"+value {
		return ""
	}
	for _, r := range value[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return value
}

func evidenceExtensionForMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "text/plain":
		return ".txt"
	case "text/markdown":
		return ".md"
	case "text/html":
		return ".html"
	case "application/json":
		return ".json"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func submittedEvidencePayload(meta SubmittedEvidenceMeta) map[string]any {
	payload := map[string]any{
		"evidence_id":         meta.EvidenceID,
		"title":               meta.Title,
		"source_url":          meta.SourceURL,
		"source_description":  meta.SourceDescription,
		"mime_type":           meta.MimeType,
		"retrieval_timestamp": meta.RetrievalTimestamp,
		"relevance":           meta.Relevance,
		"sha256":              meta.SHA256,
		"size_bytes":          meta.SizeBytes,
	}
	if meta.EvidenceID != "" {
		payload["evidence_id"] = meta.EvidenceID
	}
	return payload
}

func (rc *runContext) writeSubmittedEvidenceFile(meta SubmittedEvidenceMeta, raw []byte) (CaseFile, error) {
	dir := filepath.Join(rc.cfg.OutputDir, "submitted-evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CaseFile{}, fmt.Errorf("create submitted evidence dir: %w", err)
	}
	name := filepath.Base(meta.Name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return CaseFile{}, fmt.Errorf("invalid submitted evidence filename %q", meta.Name)
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return CaseFile{}, fmt.Errorf("submitted evidence file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return CaseFile{}, fmt.Errorf("stat submitted evidence file %s: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return CaseFile{}, fmt.Errorf("write submitted evidence %s: %w", path, err)
	}
	_, readable := caseFileKind(name)
	file := CaseFile{
		EvidenceID:   meta.EvidenceID,
		Name:         name,
		Path:         path,
		MimeType:     meta.MimeType,
		TextReadable: readable || strings.HasPrefix(strings.ToLower(meta.MimeType), "text/") || strings.EqualFold(meta.MimeType, "application/json"),
		SizeBytes:    len(raw),
	}
	if file.TextReadable {
		file.Text = string(raw)
	}
	return file, nil
}

func (rc *runContext) attorneyExhibits() []map[string]any {
	items := mapList(mapAny(rc.state["case"])["offered_evidence"])
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		evidenceID := mapString(item["evidence_id"])
		label := mapString(item["label"])
		entry := map[string]any{
			"phase":       mapString(item["phase"]),
			"role":        mapString(item["role"]),
			"evidence_id": evidenceID,
			"label":       label,
		}
		if file, ok := rc.fileByID[evidenceID]; ok {
			if file.TextReadable {
				entry["text"] = file.Text
			} else {
				entry["text"] = "(binary or non-text file)"
			}
		} else {
			entry["text"] = "(unavailable file)"
		}
		out = append(out, entry)
	}
	return out
}

func (rc *runContext) attorneyLimits(opportunity Opportunity) map[string]any {
	caseObj := mapAny(rc.state["case"])
	usedExhibits := filingCountForRole(mapList(caseObj["offered_evidence"]), opportunity.Role)
	usedReports := filingCountForRole(mapList(caseObj["technical_reports"]), opportunity.Role)
	limits := map[string]any{
		"text_char_limit": phaseTextCharLimit(rc.cfg.Policy, opportunity.Phase),
	}
	if opportunity.Phase == "arguments" || opportunity.Phase == "rebuttals" {
		limits["max_exhibits_per_filing"] = rc.cfg.Policy.MaxExhibitsPerFiling
		limits["max_exhibits_per_side"] = rc.cfg.Policy.MaxExhibitsPerSide
		limits["used_exhibits_for_side"] = usedExhibits
		limits["remaining_exhibits_for_side"] = remainingCapacity(rc.cfg.Policy.MaxExhibitsPerSide, usedExhibits)
		limits["max_reports_per_filing"] = rc.cfg.Policy.MaxReportsPerFiling
		limits["max_reports_per_side"] = rc.cfg.Policy.MaxReportsPerSide
		limits["used_reports_for_side"] = usedReports
		limits["remaining_reports_for_side"] = remainingCapacity(rc.cfg.Policy.MaxReportsPerSide, usedReports)
		usedSubmittedEvidence := submittedEvidenceCountForRole(rc.submittedEvidence, opportunity.Role)
		limits["max_submitted_evidence_per_side"] = rc.cfg.Policy.MaxSubmittedEvidencePerSide
		limits["max_submitted_evidence_bytes"] = rc.cfg.Policy.MaxSubmittedEvidenceBytes
		limits["max_direct_submitted_evidence_bytes"] = rc.cfg.Policy.MaxDirectSubmittedEvidenceBytes
		limits["max_evidence_upload_bytes"] = rc.cfg.Policy.MaxEvidenceUploadBytes
		limits["max_evidence_chunk_bytes"] = rc.cfg.Policy.MaxEvidenceChunkBytes
		limits["max_evidence_read_bytes"] = rc.cfg.Policy.MaxEvidenceReadBytes
		limits["max_evidence_reads_per_opportunity"] = rc.cfg.Policy.MaxEvidenceReadsPerOpportunity
		limits["max_evidence_read_bytes_per_opportunity"] = rc.cfg.Policy.MaxEvidenceReadBytesPerOpportunity
		limits["used_submitted_evidence_for_side"] = usedSubmittedEvidence
		limits["remaining_submitted_evidence_for_side"] = remainingCapacity(rc.cfg.Policy.MaxSubmittedEvidencePerSide, usedSubmittedEvidence)
		limits["offered_evidence_rule"] = "Use only visible case evidence_id values in offered_evidence. Submit new evidence first with aar_submit_evidence, then cite the returned evidence_id in offered_evidence."
		limits["outside_material_rule"] = "Outside source material belongs in submitted evidence when the source content matters, or in technical_reports when only attorney analysis is being offered."
	}
	if opportunity.Phase == "surrebuttals" {
		limits["outside_material_rule"] = "offered_evidence and technical_reports are not allowed in this phase."
	}
	return limits
}

func (rc *runContext) attorneyLimitsSection(opportunity Opportunity) string {
	limits := rc.attorneyLimits(opportunity)
	lines := []string{}
	if limit, _ := limits["text_char_limit"].(int); limit > 0 {
		lines = append(lines, fmt.Sprintf("Text limit for this submission: %d characters.", limit))
		lines = append(lines, fmt.Sprintf("Target length for the first submission: %d characters or less.", targetSubmissionCharLimit(limit)))
	}
	switch opportunity.Phase {
	case "arguments", "rebuttals":
		lines = append(lines,
			fmt.Sprintf(
				"Exhibits: at most %d in this filing. This side has used %d of %d total, with %d left.",
				limits["max_exhibits_per_filing"].(int),
				limits["used_exhibits_for_side"].(int),
				limits["max_exhibits_per_side"].(int),
				limits["remaining_exhibits_for_side"].(int),
			),
		)
		lines = append(lines,
			fmt.Sprintf(
				"Technical reports: at most %d in this filing. This side has used %d of %d total, with %d left.",
				limits["max_reports_per_filing"].(int),
				limits["used_reports_for_side"].(int),
				limits["max_reports_per_side"].(int),
				limits["remaining_reports_for_side"].(int),
			),
		)
		lines = append(lines,
			fmt.Sprintf(
				"Submitted evidence: admitted items may be at most %d bytes. Direct aar_submit_evidence items may be at most %d bytes; chunked evidence uploads may be at most %d bytes with %d-byte chunks. This side has submitted %d of %d total, with %d left.",
				limits["max_submitted_evidence_bytes"].(int),
				limits["max_direct_submitted_evidence_bytes"].(int),
				limits["max_evidence_upload_bytes"].(int),
				limits["max_evidence_chunk_bytes"].(int),
				limits["used_submitted_evidence_for_side"].(int),
				limits["max_submitted_evidence_per_side"].(int),
				limits["remaining_submitted_evidence_for_side"].(int),
			),
		)
		lines = append(lines, fmt.Sprintf("Evidence reads: at most %d bytes per read, %d reads per opportunity, and %d bytes total per opportunity.", limits["max_evidence_read_bytes"].(int), limits["max_evidence_reads_per_opportunity"].(int), limits["max_evidence_read_bytes_per_opportunity"].(int)))
		lines = append(lines, "Use only visible case evidence_id values in offered_evidence. Submit new source material first with aar_submit_evidence, then cite the returned evidence_id in offered_evidence. Use evidence_id and hash for custody checks and exact byte inspection.")
		lines = append(lines, "Use technical_reports for attorney analysis or synthesized work product, not as a substitute for source evidence when exact source content matters.")
	case "surrebuttals":
		lines = append(lines, "offered_evidence and technical_reports are not allowed in this phase.")
	}
	return strings.Join(lines, "\n")
}

func targetSubmissionCharLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	target := (limit * 3) / 4
	if target <= 0 {
		target = limit
	}
	return target
}

func phaseTextCharLimit(policy Policy, phase string) int {
	switch phase {
	case "openings":
		return policy.MaxOpeningChars
	case "arguments":
		return policy.MaxArgumentChars
	case "rebuttals":
		return policy.MaxRebuttalChars
	case "surrebuttals":
		return policy.MaxSurrebuttalChars
	case "closings":
		return policy.MaxClosingChars
	default:
		return 0
	}
}

func filingCountForRole(items []map[string]any, role string) int {
	count := 0
	for _, item := range items {
		if mapString(item["role"]) == role {
			count++
		}
	}
	return count
}

func submittedEvidenceCountForRole(items []SubmittedEvidenceMeta, role string) int {
	count := 0
	for _, item := range items {
		if item.Role == role {
			count++
		}
	}
	return count
}

func remainingCapacity(limit int, used int) int {
	if limit-used < 0 {
		return 0
	}
	return limit - used
}

func (rc *runContext) validateAttorneyPayloadAgainstState(opportunity Opportunity, actionType string, payload map[string]any) error {
	text := strings.TrimSpace(mapString(payload["text"]))
	if limit := phaseTextCharLimit(rc.cfg.Policy, opportunity.Phase); limit > 0 {
		charCount := len([]rune(text))
		if charCount > limit {
			return fmt.Errorf("%s exceeds character limit of %d (got %d)", filingLabel(actionType), limit, charCount)
		}
	}
	switch actionType {
	case "submit_argument", "submit_rebuttal":
		caseObj := mapAny(rc.state["case"])
		usedExhibits := filingCountForRole(mapList(caseObj["offered_evidence"]), opportunity.Role)
		attemptedExhibits := len(listOfMaps(payload["offered_evidence"]))
		if usedExhibits+attemptedExhibits > rc.cfg.Policy.MaxExhibitsPerSide {
			return fmt.Errorf(
				"offered_evidence for this side exceed limit of %d (%d already used, %d attempted, %d remaining)",
				rc.cfg.Policy.MaxExhibitsPerSide,
				usedExhibits,
				attemptedExhibits,
				remainingCapacity(rc.cfg.Policy.MaxExhibitsPerSide, usedExhibits),
			)
		}
		usedReports := filingCountForRole(mapList(caseObj["technical_reports"]), opportunity.Role)
		attemptedReports := len(listOfMaps(payload["technical_reports"]))
		if usedReports+attemptedReports > rc.cfg.Policy.MaxReportsPerSide {
			return fmt.Errorf(
				"technical_reports for this side exceed limit of %d (%d already used, %d attempted, %d remaining)",
				rc.cfg.Policy.MaxReportsPerSide,
				usedReports,
				attemptedReports,
				remainingCapacity(rc.cfg.Policy.MaxReportsPerSide, usedReports),
			)
		}
	}
	return nil
}

func filingLabel(actionType string) string {
	switch actionType {
	case "record_opening_statement":
		return "opening statement"
	case "submit_argument":
		return "argument"
	case "submit_rebuttal":
		return "rebuttal"
	case "submit_surrebuttal":
		return "surrebuttal"
	case "deliver_closing_statement":
		return "closing statement"
	default:
		return "submission"
	}
}

func marshalInline(value any) string {
	wire, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(wire)
}

func marshalIndented(value any) string {
	wire, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(wire)
}

func copyTree(dstRoot string, srcRoot string) error {
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}
		dstPath := dstRoot
		if rel != "." {
			dstPath = filepath.Join(dstRoot, rel)
		}
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", dstPath, err)
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink work product is not allowed: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported work-product entry %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := os.WriteFile(dstPath, raw, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", dstPath, err)
		}
		return nil
	})
}
