package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"adjudication/adc/runtime/spec"
	"adjudication/common/acp"
)

type ACPRoleConfig struct {
	Role    string
	Command string
	Args    []string
	Env     []string
	Timeout time.Duration
}

type ACPConfig struct {
	Roles   map[string]struct{}
	Command string
	Args    []string
	Env     []string
	Timeout time.Duration
}

type acpPersistentSession struct {
	client      *acp.Client
	sessionPath string
	cleanup     func() error
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

func NewACPConfig(roles []string, command string, args []string, env []string, timeout time.Duration) (*ACPConfig, error) {
	roleSet := map[string]struct{}{}
	for _, raw := range roles {
		role := strings.TrimSpace(raw)
		if role == "" {
			continue
		}
		roleSet[role] = struct{}{}
	}
	if len(roleSet) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("acp command is required")
	}
	return &ACPConfig{
		Roles:   roleSet,
		Command: strings.TrimSpace(command),
		Args:    append([]string{}, args...),
		Env:     append([]string{}, env...),
		Timeout: timeout,
	}, nil
}

func (cfg ACPRoleConfig) sharedConfig() ACPConfig {
	return ACPConfig{
		Roles:   map[string]struct{}{strings.TrimSpace(cfg.Role): struct{}{}},
		Command: strings.TrimSpace(cfg.Command),
		Args:    append([]string{}, cfg.Args...),
		Env:     append([]string{}, cfg.Env...),
		Timeout: cfg.Timeout,
	}
}

func (r *Runner) RunACPRoleExperiment(ctx context.Context, cfg ACPRoleConfig) (Result, error) {
	if strings.TrimSpace(cfg.Role) == "" {
		return Result{}, fmt.Errorf("acp role is required")
	}
	if strings.TrimSpace(cfg.Command) == "" {
		return Result{}, fmt.Errorf("acp command is required")
	}
	if err := resetEventLog(r.cfg.EventsPath); err != nil {
		return Result{}, err
	}
	if err := r.store.CreateRun(r.cfg.RunID, r.scenario.Name); err != nil {
		return Result{}, err
	}
	turnLogs := make([]TurnLog, 0, len(r.scenario.Turns)+1)
	for i, turn := range r.scenario.Turns {
		role, ok := r.roles[turn.Role]
		if !ok {
			return Result{}, fmt.Errorf("unknown role: %s", turn.Role)
		}
		if turn.DeterministicAction == nil {
			return Result{}, fmt.Errorf("acp role experiment requires deterministic scenario prefix; turn %d role=%s is live", i+1, role.Name)
		}
		allowed := turn.EffectiveAllowedActions(role)
		if len(allowed) == 0 {
			return Result{}, fmt.Errorf("turn %d role=%s has no allowed actions", i+1, role.Name)
		}
		log, err := r.executeTurn(ctx, i+1, role, turn, allowed)
		if err != nil {
			return Result{}, err
		}
		turnLogs = append(turnLogs, log)
	}
	loop := r.scenario.LoopPolicy
	if loop == nil || strings.TrimSpace(loop.Type) != "autopilot_trial" {
		return Result{}, fmt.Errorf("acp role experiment requires loop_policy.type=autopilot_trial")
	}
	rolesPayload := r.autopilotRolesPayload()
	resp, err := r.lean.NextOpportunity(r.state, rolesPayload, loop.MaxStepsPerTurn)
	if err != nil {
		return Result{}, fmt.Errorf("lean next_opportunity failed: %w", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return Result{}, fmt.Errorf("lean next_opportunity error: %s", stringOrDefault(resp["error"], "unknown error"))
	}
	if terminal, _ := resp["terminal"].(bool); terminal {
		return Result{}, fmt.Errorf("lean returned terminal opportunity state before external role acted")
	}
	stateVersion := intFromAny(resp["state_version"])
	opportunityPayload, _ := resp["opportunity"].(map[string]any)
	if len(opportunityPayload) == 0 {
		return Result{}, fmt.Errorf("lean next_opportunity returned empty opportunity")
	}
	opportunity, err := parseLeanOpportunity(opportunityPayload)
	if err != nil {
		return Result{}, err
	}
	if opportunity.Role != strings.TrimSpace(cfg.Role) {
		return Result{}, fmt.Errorf("next opportunity belongs to %s, not requested role %s", opportunity.Role, cfg.Role)
	}
	role, ok := r.roles[opportunity.Role]
	if !ok {
		return Result{}, fmt.Errorf("unknown role: %s", opportunity.Role)
	}
	fmt.Fprintf(
		os.Stderr,
		"agent call turn=%d source=acp_role role=%s opportunity_id=%s phase=%s kind=%s may_pass=%t why=%s allowed=%s\n",
		len(turnLogs)+1,
		role.Name,
		opportunity.OpportunityID,
		opportunity.Phase,
		opportunity.Kind,
		opportunity.MayPass,
		opportunity.Objective,
		strings.Join(opportunity.AllowedTools, ","),
	)
	log, err := r.executeOpportunityTurnACP(ctx, len(turnLogs)+1, role, opportunity, rolesPayload, stateVersion, cfg.sharedConfig())
	if err != nil {
		if cleanupErr := r.closeACPSessions(); cleanupErr != nil {
			return Result{}, errors.Join(err, cleanupErr)
		}
		return Result{}, err
	}
	log.Source = "acp_role"
	log.ActionID = opportunity.OpportunityID
	turnLogsApplyOpportunity(len(turnLogs)+1, &log, opportunity)
	turnLogs = append(turnLogs, log)

	if err := r.closeACPSessions(); err != nil {
		return Result{}, err
	}

	assertions := evaluateAssertions(r.scenario.Assertions, r.state, turnLogs)
	result := Result{
		Scenario:   r.scenario.Name,
		Assertions: assertions,
		TurnLogs:   turnLogs,
		FinalState: r.state,
	}
	if err := r.writeArtifacts(result); err != nil {
		return Result{}, err
	}
	status := "ok"
	for _, a := range assertions {
		if passed, _ := a["passed"].(bool); !passed {
			status = "assertion_failed"
			break
		}
	}
	artifactMap := map[string]any{}
	if raw, err := json.Marshal(result); err == nil {
		_ = json.Unmarshal(raw, &artifactMap)
	}
	if err := r.store.FinishRun(r.cfg.RunID, status, r.state, artifactMap); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r *Runner) ensureACPSession(ctx context.Context, role spec.RoleSpec, cfg ACPConfig, sessionCwd string) (*acpPersistentSession, error) {
	roleName := strings.TrimSpace(role.Name)
	if roleName == "" {
		return nil, fmt.Errorf("acp role name is required")
	}
	if session, ok := r.acpSessions[roleName]; ok {
		return session, nil
	}
	sessionACPPath := sessionCwd
	cleanup := func() error { return nil }
	containerHomeDir := ""
	if usesPIContainerWrapper(cfg.Command) {
		commandPath := strings.TrimSpace(cfg.Command)
		if !filepath.IsAbs(commandPath) {
			var err error
			commandPath, err = filepath.Abs(commandPath)
			if err != nil {
				return nil, fmt.Errorf("resolve ACP command path: %w", err)
			}
		}
		repoRoot := filepath.Dir(filepath.Dir(commandPath))
		var err error
		containerHomeDir, cleanup, err = prepareEphemeralPIHome(repoRoot)
		if err != nil {
			return nil, err
		}
		sessionACPPath = "/home/user"
	}
	toolSpecs := acpRoleToolSpecs(role)
	env := append([]string{}, cfg.Env...)
	env = append(env, "PI_ACP_CLIENT_TOOLS="+marshalString(toolSpecs))
	if containerHomeDir != "" {
		env = append(env, "PI_CONTAINER_HOME_DIR="+containerHomeDir)
	}
	client, err := acp.NewClient(acp.Config{
		Command: cfg.Command,
		Args:    cfg.Args,
		Cwd:     sessionCwd,
		Env:     env,
	})
	if err != nil {
		return nil, errors.Join(err, cleanup())
	}
	session := &acpPersistentSession{
		client:  client,
		cleanup: cleanup,
	}
	if _, err := client.Initialize(ctx, 1); err != nil {
		return nil, errors.Join(err, session.Close())
	}
	session.sessionPath = sessionACPPath
	r.acpSessions[roleName] = session
	return session, nil
}

func (r *Runner) closeACPSessions() error {
	if len(r.acpSessions) == 0 {
		return nil
	}
	roleNames := make([]string, 0, len(r.acpSessions))
	for roleName := range r.acpSessions {
		roleNames = append(roleNames, roleName)
	}
	sort.Strings(roleNames)
	var err error
	for _, roleName := range roleNames {
		session := r.acpSessions[roleName]
		delete(r.acpSessions, roleName)
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ACP session role=%s: %w", roleName, closeErr))
		}
	}
	return err
}

func (r *Runner) executeOpportunityTurnACP(
	ctx context.Context,
	turnIndex int,
	role spec.RoleSpec,
	opportunity leanOpportunity,
	rolesPayload []map[string]any,
	stateVersion int,
	cfg ACPConfig,
) (TurnLog, error) {
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	sessionCwd := r.cfg.ScenarioBaseDir
	if sessionCwd == "" {
		var err error
		sessionCwd, err = os.Getwd()
		if err != nil {
			return TurnLog{}, fmt.Errorf("get cwd: %w", err)
		}
	}
	sessionCwd, err := filepath.Abs(sessionCwd)
	if err != nil {
		return TurnLog{}, fmt.Errorf("resolve scenario cwd: %w", err)
	}
	session, err := r.ensureACPSession(ctx, role, cfg, sessionCwd)
	if err != nil {
		return TurnLog{}, err
	}
	client := session.client
	sessionResp, err := client.NewSession(ctx, session.sessionPath)
	if err != nil {
		return TurnLog{}, err
	}
	if strings.TrimSpace(sessionResp.SessionID) == "" {
		return TurnLog{}, fmt.Errorf("acp session/new returned empty session id")
	}

	transcript := make([]map[string]any, 0)
	var transcriptMu sync.Mutex
	appendTranscript := func(entry map[string]any) {
		transcriptMu.Lock()
		transcript = append(transcript, entry)
		transcriptMu.Unlock()
	}
	transcriptSnapshot := func() []map[string]any {
		transcriptMu.Lock()
		defer transcriptMu.Unlock()
		out := make([]map[string]any, len(transcript))
		copy(out, transcript)
		return out
	}
	var notifyErr error
	var notifyErrMu sync.Mutex
	setNotifyErr := func(err error) {
		if err == nil {
			return
		}
		notifyErrMu.Lock()
		if notifyErr == nil {
			notifyErr = err
		}
		notifyErrMu.Unlock()
	}
	getNotifyErr := func() error {
		notifyErrMu.Lock()
		defer notifyErrMu.Unlock()
		return notifyErr
	}
	agentEventSequence := 0
	lastAgentToolStatus := map[string]string{}
	recordAgentEvent := func(eventType string, payload map[string]any) {
		agentEventSequence++
		if err := r.persistAgentEvent(turnIndex, agentEventSequence, role.Name, eventType, payload); err != nil {
			setNotifyErr(err)
		}
	}
	view, err := r.lean.View(r.state, role.Name)
	if err != nil {
		return TurnLog{}, err
	}
	stepsUsed := 0
	decisionStepsUsed := 0
	supportStepsUsed := 0
	supportBudget := supportToolBudget(r.state)
	decisionSubmitted := false

	recordActionStep := func() int {
		stepsUsed++
		return stepsUsed
	}

	callAction := func(actionType string, payload map[string]any) (ActionExecution, error) {
		if supportStepsUsed >= supportBudget {
			return ActionExecution{}, fmt.Errorf("You have inspected enough record material for this opportunity.  Submit a legal decision now, or pass if passing is allowed.")
		}
		supportStepsUsed++
		stepIndex := recordActionStep()
		execRes, err := r.executeAction(turnIndex, stepIndex, role.Name, actionType, payload)
		if err != nil {
			return ActionExecution{}, err
		}
		res := execRes.Result
		appendTranscript(map[string]any{
			"custom_method": acpCustomMethod(acpMethodForAction(actionType)),
			"action":        actionType,
			"arguments":     payload,
			"result":        res,
		})
		if ok, _ := res["ok"].(bool); !ok {
			return ActionExecution{}, fmt.Errorf("%s", issueText(issueFromResult(actionType, res)))
		}
		return execRes, nil
	}

	client.HandleMethod(acpCustomMethod("get_case"), func(_ context.Context, _ map[string]any) (map[string]any, error) {
		execRes, err := callAction("get_case", map[string]any{})
		if err != nil {
			return nil, err
		}
		res := execRes.Result
		return map[string]any{
			"text": marshalString(map[string]any{"case": res["case"]}),
			"case": res["case"],
		}, nil
	})
	client.HandleMethod(acpCustomMethod("list_case_files"), func(_ context.Context, _ map[string]any) (map[string]any, error) {
		execRes, err := callAction("list_case_files", map[string]any{})
		if err != nil {
			return nil, err
		}
		res := execRes.Result
		return map[string]any{
			"text":  marshalString(map[string]any{"files": res["files"]}),
			"files": res["files"],
		}, nil
	})
	client.HandleMethod(acpCustomMethod("read_case_text_file"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		payload := map[string]any{
			"file_id": strings.TrimSpace(stringOrDefault(params["file_id"], "")),
		}
		execRes, err := callAction("read_case_text_file", payload)
		if err != nil {
			return nil, err
		}
		res := execRes.Result
		return map[string]any{
			"text":    stringOrDefault(res["text"], ""),
			"file_id": payload["file_id"],
		}, nil
	})
	client.HandleMethod(acpCustomMethod("request_case_file"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		payload := map[string]any{
			"file_id": strings.TrimSpace(stringOrDefault(params["file_id"], "")),
		}
		execRes, err := callAction("request_case_file", payload)
		if err != nil {
			return nil, err
		}
		content, err := acpContentFromFollowupItems(execRes.FollowupInputItems)
		if err != nil {
			return nil, err
		}
		res := execRes.Result
		out := map[string]any{
			"content":  content,
			"file":     res["file"],
			"attached": res["attached"],
		}
		if len(content) == 0 {
			out["text"] = "Requested case file attached."
		}
		return out, nil
	})
	client.HandleMethod(acpCustomMethod("get_juror_context"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		payload := map[string]any{
			"juror_id": strings.TrimSpace(stringOrDefault(params["juror_id"], "")),
		}
		execRes, err := callAction("get_juror_context", payload)
		if err != nil {
			return nil, err
		}
		res := execRes.Result
		return map[string]any{
			"text":    marshalString(map[string]any{"context": res["context"]}),
			"context": res["context"],
		}, nil
	})
	client.HandleMethod(acpCustomMethod("submit_decision"), func(_ context.Context, params map[string]any) (map[string]any, error) {
		if decisionSubmitted {
			return nil, fmt.Errorf("a decision has already been accepted for this opportunity")
		}
		if decisionStepsUsed >= opportunity.StepBudget {
			return nil, fmt.Errorf("This opportunity's decision budget is exhausted.")
		}
		decisionStepsUsed++
		stepIndex := recordActionStep()
		decision, err := acpDecisionFromParams(params)
		if err != nil {
			return nil, err
		}
		acceptResp, err := r.lean.ApplyDecision(r.state, stateVersion, opportunity.OpportunityID, role.Name, decision, rolesPayload, opportunity.StepBudget)
		if err != nil {
			return nil, err
		}
		appendTranscript(map[string]any{
			"custom_method": acpCustomMethod("submit_decision"),
			"decision":      decision,
			"acceptance":    acceptResp,
		})
		if ok, _ := acceptResp["ok"].(bool); !ok {
			return nil, fmt.Errorf("%s", issueText(issueFromResult("submit_decision", acceptResp)))
		}
		resultKind := strings.TrimSpace(stringOrDefault(acceptResp["result_kind"], ""))
		switch resultKind {
		case "pass_recorded":
			state, _ := acceptResp["state"].(map[string]any)
			if state == nil {
				return nil, fmt.Errorf("lean apply_decision pass_recorded missing state")
			}
			r.state = mergeLocalCaseExtensions(r.state, state)
			if err := r.persistActionEvent(turnIndex, stepIndex, role.Name, "pass_turn", params, acceptResp); err != nil {
				return nil, err
			}
			decisionSubmitted = true
			appendTranscript(map[string]any{"action": "pass_turn", "arguments": params, "result": acceptResp})
			return map[string]any{
				"text":       "Decision accepted. Pass recorded.",
				"acceptance": acceptResp,
			}, nil
		case "execute_tool":
			action, _ := acceptResp["action"].(map[string]any)
			if action == nil {
				return nil, fmt.Errorf("lean apply_decision execute_tool missing action")
			}
			actionType := strings.TrimSpace(stringOrDefault(action["action_type"], ""))
			actorRole := strings.TrimSpace(stringOrDefault(action["actor_role"], role.Name))
			payload, _ := action["payload"].(map[string]any)
			if payload == nil {
				payload = map[string]any{}
			}
			execRes, err := r.executeAction(turnIndex, stepIndex, actorRole, actionType, payload)
			if err != nil {
				return nil, err
			}
			res := execRes.Result
			appendTranscript(map[string]any{"action": actionType, "arguments": payload, "result": res})
			if ok, _ := res["ok"].(bool); !ok {
				return nil, fmt.Errorf("%s", issueText(issueFromResult(actionType, res)))
			}
			decisionSubmitted = true
			return map[string]any{
				"text":       "Decision accepted. Executed " + actionType + ".",
				"acceptance": acceptResp,
				"result":     res,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported apply_decision result_kind: %s", resultKind)
		}
	})

	unsub := client.OnNotification(func(note acp.Notification) {
		if note.Method != "session/update" {
			return
		}
		update, _ := note.Params["update"].(map[string]any)
		if update == nil {
			return
		}
		switch stringOrDefault(update["sessionUpdate"], "") {
		case "agent_message_chunk", "agent_thought_chunk":
			content, _ := update["content"].(map[string]any)
			text := strings.TrimSpace(stringOrDefault(content["text"], ""))
			if text != "" {
				appendTranscript(map[string]any{"assistant_text": text})
				_, _ = fmt.Fprintln(os.Stderr, text)
			}
		case "tool_call":
			entry := map[string]any{
				"tool_call_id": stringOrDefault(update["toolCallId"], ""),
				"title":        stringOrDefault(update["title"], ""),
				"status":       stringOrDefault(update["status"], ""),
				"raw_input":    update["rawInput"],
			}
			if toolCallID := strings.TrimSpace(stringOrDefault(entry["tool_call_id"], "")); toolCallID != "" {
				lastAgentToolStatus[toolCallID] = strings.TrimSpace(stringOrDefault(entry["status"], ""))
			}
			appendTranscript(map[string]any{"agent_tool_call": entry})
			recordAgentEvent("agent_tool_call", entry)
		case "tool_call_update":
			entry := map[string]any{
				"tool_call_id": stringOrDefault(update["toolCallId"], ""),
				"status":       stringOrDefault(update["status"], ""),
			}
			if rawInput := update["rawInput"]; rawInput != nil {
				entry["raw_input"] = rawInput
			}
			if rawOutput := update["rawOutput"]; rawOutput != nil {
				entry["raw_output"] = sanitizeACPToolRawOutput(rawOutput)
			}
			toolCallID := strings.TrimSpace(stringOrDefault(entry["tool_call_id"], ""))
			status := strings.TrimSpace(stringOrDefault(entry["status"], ""))
			if toolCallID != "" &&
				entry["raw_input"] == nil &&
				entry["raw_output"] == nil &&
				status != "" &&
				lastAgentToolStatus[toolCallID] == status {
				return
			}
			if toolCallID != "" && status != "" {
				lastAgentToolStatus[toolCallID] = status
			}
			appendTranscript(map[string]any{"agent_tool_update": entry})
			recordAgentEvent("agent_tool_update", entry)
		}
	})
	defer unsub()

	promptText := r.buildACPRolePrompt(role, view, opportunity)
	_, err = client.Prompt(ctx, acp.PromptRequest{
		SessionID: sessionResp.SessionID,
		Prompt:    []acp.TextBlock{{Type: "text", Text: promptText}},
	})
	if err != nil {
		if err := getNotifyErr(); err != nil {
			return TurnLog{}, err
		}
		if decisionSubmitted {
			return TurnLog{
				Role:       role.Name,
				Prompt:     opportunity.Objective,
				Steps:      stepsUsed,
				Transcript: transcriptSnapshot(),
			}, nil
		}
		return TurnLog{}, err
	}
	if err := getNotifyErr(); err != nil {
		return TurnLog{}, err
	}
	if !decisionSubmitted {
		return TurnLog{}, fmt.Errorf("external ACP role did not submit a decision")
	}
	return TurnLog{
		Role:       role.Name,
		Prompt:     opportunity.Objective,
		Steps:      stepsUsed,
		Transcript: transcriptSnapshot(),
	}, nil
}

func acpRoleToolSpecs(role spec.RoleSpec) []map[string]any {
	specs := make([]map[string]any, 0, 4)
	for _, name := range referenceToolsForRole(role) {
		switch name {
		case "get_case":
			specs = append(specs, map[string]any{
				"method":      acpCustomMethod("get_case"),
				"toolName":    acpToolName("get_case"),
				"description": "Return the current role-visible case view as JSON text.",
				"parameters":  map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			})
		case "list_case_files":
			specs = append(specs, map[string]any{
				"method":      acpCustomMethod("list_case_files"),
				"toolName":    acpToolName("list_case_files"),
				"description": "Return visible case files with file identifiers and metadata.",
				"parameters":  map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			})
		case "read_case_text_file":
			specs = append(specs, map[string]any{
				"method":      acpCustomMethod("read_case_text_file"),
				"toolName":    acpToolName("read_case_text_file"),
				"description": "Read a visible .txt, .md, .pem, or .b64 case file by file_id.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_id": map[string]any{"type": "string", "description": "Visible case file identifier"},
					},
					"required":             []string{"file_id"},
					"additionalProperties": false,
				},
			})
		case "request_case_file":
			specs = append(specs, map[string]any{
				"method":      acpCustomMethod("request_case_file"),
				"toolName":    acpToolName("request_case_file"),
				"description": "Attach one visible case file for inspection. Images are attached as image content.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_id": map[string]any{"type": "string", "description": "Visible case file identifier"},
					},
					"required":             []string{"file_id"},
					"additionalProperties": false,
				},
			})
		case "get_juror_context":
			specs = append(specs, map[string]any{
				"method":      acpCustomMethod("get_juror_context"),
				"toolName":    acpToolName("get_juror_context"),
				"description": "Return the questionnaire answers and oral voir dire record for one juror candidate by juror_id.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"juror_id": map[string]any{"type": "string", "description": "Prospective juror identifier"},
					},
					"required":             []string{"juror_id"},
					"additionalProperties": false,
				},
			})
		}
	}
	specs = append(specs, map[string]any{
		"method":      acpCustomMethod("submit_decision"),
		"toolName":    acpToolName("submit_decision"),
		"description": "Submit one legal decision for the current opportunity. Use kind=tool for an allowed tool or kind=pass if passing is allowed.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":      map[string]any{"type": "string", "enum": []string{"tool", "pass"}},
				"tool_name": map[string]any{"type": "string"},
				"payload":   map[string]any{"type": "object"},
				"reason":    map[string]any{"type": "string"},
			},
			"required":             []string{"kind"},
			"additionalProperties": false,
		},
	})
	return specs
}

func (r *Runner) buildACPRolePrompt(role spec.RoleSpec, view map[string]any, opportunity leanOpportunity) string {
	lines := []string{
		buildSystemPrompt(role, view),
		"",
		buildOpportunityPrompt(role, opportunity),
		"",
		"Allowed legal tools for submit_decision: " + strings.Join(opportunity.AllowedTools, ", "),
		"When kind=tool, adc_submit_decision takes exactly three fields: kind, tool_name, and payload.",
		"Put every legal tool argument inside payload.  Do not put legal tool fields at the top level of adc_submit_decision.",
	}
	if schemaLines := r.legalToolSchemaLines(opportunity.AllowedTools); len(schemaLines) > 0 {
		lines = append(lines, "", "Legal tool payloads:")
		lines = append(lines, schemaLines...)
	}
	if examples := legalDecisionExamples(opportunity.AllowedTools); len(examples) > 0 {
		lines = append(lines, "", "Decision examples:")
		lines = append(lines, examples...)
	}
	if len(opportunity.Constraints) > 0 {
		lines = append(lines, "Opportunity constraints: "+marshalString(opportunity.Constraints))
	}
	if opportunity.MayPass {
		lines = append(lines, "Passing is allowed.  Use adc_submit_decision with kind=pass if you decline to act.")
	} else {
		lines = append(lines, "You must submit one legal decision in this opportunity.")
	}
	lines = append(lines, "", "Available host tools:")
	for _, line := range acpHostToolDescriptions(role) {
		lines = append(lines, line)
	}
	lines = append(lines,
		"",
		"Case files are not attached automatically.",
		"You may use local tools in your runtime environment to analyze materials you obtain through the ADC tools.",
		"If a needed local tool is missing, you may install it in that runtime environment for the current task.",
		"You already have the current visible case view above.  Do not call adc_get_case to reread the same view.",
		"To inspect a visible image, call adc_list_case_files, then adc_request_case_file with the chosen file_id.",
		"To inspect a visible .txt, .md, .pem, or .b64 file, call adc_list_case_files, then adc_read_case_text_file with the chosen file_id.",
		"Before you submit a technical report, trial theory, exhibit offer, motion, opening, or closing, analyze the visible case files that bear on the disputed points.",
		"Do the analysis before you draft the filing, not as a plan for later.",
		"If a visible file permits a concrete check, calculation, or verification, perform it and state the result.",
		"Do not submit a technical report, motion, opening, closing, or trial theory that only proposes a later verification or calculation when you can do it now from the visible case files.",
		"If you cannot execute a relevant check from the visible case files, say exactly what is missing.",
		"For example: if the visible files include a text statement, a detached signature in .b64 form, and a public key in .pem form, read those files, decode the signature locally, verify it locally, and report the result instead of saying verification could be done later.",
		"You do not have direct filesystem access to case materials.",
		"If you need to add a new file to the case, submit import_case_file with original_name and content_base64 in the payload.  Do not refer to a host path.",
	)
	if opportunity.Phase == "voir_dire" && contains(referenceToolsForRole(role), "get_juror_context") {
		lines = append(lines,
			"For a voir dire opportunity tied to one named juror, use adc_get_juror_context with that juror_id instead of adc_get_case.",
		)
	}
	lines = append(lines, "",
		"When you have successfully submitted a decision, reply exactly: decision-submitted.",
		"Do not describe a legal act in prose.  Use adc_submit_decision to perform it.",
	)
	return strings.Join(lines, "\n")
}

func legalDecisionExamples(allowedTools []string) []string {
	examples := make([]string, 0, len(allowedTools)+1)
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		example := map[string]any{
			"kind":      "tool",
			"tool_name": toolName,
			"payload":   map[string]any{},
		}
		examples = append(examples, "- "+toolName+": "+marshalString(example))
	}
	examples = append(examples, "- pass: "+marshalString(map[string]any{"kind": "pass", "reason": "brief explanation"}))
	return examples
}

func (r *Runner) legalToolSchemaLines(allowedTools []string) []string {
	lines := make([]string, 0, len(allowedTools))
	seen := map[string]bool{}
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		schema := r.toolSchema(toolName)
		if schema == nil {
			continue
		}
		lines = append(lines, "- "+toolName+": "+marshalString(schema))
	}
	return lines
}

func acpHostToolDescriptions(role spec.RoleSpec) []string {
	lines := make([]string, 0, 5)
	for _, name := range referenceToolsForRole(role) {
		switch name {
		case "get_case":
			lines = append(lines, "- adc_get_case: fetch the current visible case view.")
		case "list_case_files":
			lines = append(lines, "- adc_list_case_files: list visible file_id values and metadata.")
		case "read_case_text_file":
			lines = append(lines, "- adc_read_case_text_file: read a visible .txt, .md, .pem, or .b64 file by file_id.")
		case "request_case_file":
			lines = append(lines, "- adc_request_case_file: attach one visible case file for inspection.  Use this for images.")
		case "get_juror_context":
			lines = append(lines, "- adc_get_juror_context: fetch one juror candidate's questionnaire answers and oral voir dire record by juror_id.")
		}
	}
	lines = append(lines, "- adc_submit_decision: submit the actual legal act for this opportunity.")
	return lines
}

func acpDecisionFromParams(params map[string]any) (map[string]any, error) {
	kind := strings.TrimSpace(stringOrDefault(params["kind"], ""))
	switch kind {
	case "pass":
		return map[string]any{
			"kind":   "pass",
			"reason": strings.TrimSpace(stringOrDefault(params["reason"], "")),
		}, nil
	case "tool":
		toolName := strings.TrimSpace(stringOrDefault(params["tool_name"], ""))
		if toolName == "" {
			return nil, fmt.Errorf("submit_decision requires tool_name when kind=tool")
		}
		payload, _ := params["payload"].(map[string]any)
		if payload == nil {
			payload = map[string]any{}
		}
		return map[string]any{
			"kind":      "tool",
			"tool_name": toolName,
			"payload":   payload,
		}, nil
	default:
		return nil, fmt.Errorf("submit_decision kind must be tool or pass")
	}
}

func acpCustomMethod(name string) string {
	return "_adc/" + strings.TrimSpace(name)
}

func acpToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return "adc_" + strings.ReplaceAll(name, "/", "_")
}

func acpMethodForAction(actionType string) string {
	switch actionType {
	case "get_case":
		return "get_case"
	case "list_case_files":
		return "list_case_files"
	case "read_case_text_file":
		return "read_case_text_file"
	case "request_case_file":
		return "request_case_file"
	case "get_juror_context":
		return "get_juror_context"
	default:
		return actionType
	}
}

func acpContentFromFollowupItems(items []map[string]any) ([]map[string]any, error) {
	content := make([]map[string]any, 0)
	for _, item := range items {
		rawItems, _ := item["content_items"].([]map[string]any)
		if rawItems == nil {
			rawAny, _ := item["content_items"].([]any)
			for _, raw := range rawAny {
				entry, _ := raw.(map[string]any)
				if entry != nil {
					rawItems = append(rawItems, entry)
				}
			}
		}
		for _, entry := range rawItems {
			kind := strings.TrimSpace(stringOrDefault(entry["type"], ""))
			switch kind {
			case "input_text":
				text := stringOrDefault(entry["text"], "")
				if text != "" {
					content = append(content, map[string]any{"type": "text", "text": text})
				}
			case "input_image":
				imageURL := strings.TrimSpace(stringOrDefault(entry["image_url"], ""))
				mimeType, data, err := parseDataURL(imageURL)
				if err != nil {
					return nil, err
				}
				content = append(content, map[string]any{"type": "image", "data": data, "mimeType": mimeType})
			case "input_file":
				filename := strings.TrimSpace(stringOrDefault(entry["filename"], ""))
				if filename == "" {
					filename = "case file"
				}
				return nil, fmt.Errorf("request_case_file via ACP supports images only at present; %s is not an image attachment", filename)
			}
		}
	}
	return content, nil
}

func parseDataURL(value string) (string, string, error) {
	if !strings.HasPrefix(value, "data:") {
		return "", "", fmt.Errorf("expected data URL image attachment")
	}
	rest := strings.TrimPrefix(value, "data:")
	parts := strings.SplitN(rest, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed data URL image attachment")
	}
	header := parts[0]
	data := parts[1]
	headerParts := strings.Split(header, ";")
	if len(headerParts) < 2 || headerParts[len(headerParts)-1] != "base64" {
		return "", "", fmt.Errorf("data URL image attachment is not base64 encoded")
	}
	mimeType := strings.TrimSpace(headerParts[0])
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if strings.TrimSpace(data) == "" {
		return "", "", fmt.Errorf("data URL image attachment is empty")
	}
	return mimeType, data, nil
}

func issueText(issue correctionIssue) string {
	if strings.TrimSpace(issue.ActorMessage) != "" {
		return strings.TrimSpace(issue.ActorMessage)
	}
	if strings.TrimSpace(issue.Error) != "" {
		return strings.TrimSpace(issue.Error)
	}
	return "request failed"
}

func (r *Runner) externalACPConfigForRole(roleName string) *ACPConfig {
	cfg := r.cfg.ACP
	if cfg == nil {
		return nil
	}
	if _, ok := cfg.Roles[strings.TrimSpace(roleName)]; !ok {
		return nil
	}
	return cfg
}

func sanitizeACPToolRawOutput(raw any) any {
	switch value := raw.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, inner := range value {
			if key == "content" {
				if content, ok := inner.([]any); ok {
					out[key] = sanitizeACPContent(content)
					continue
				}
			}
			out[key] = sanitizeACPToolRawOutput(inner)
		}
		return out
	case []any:
		out := make([]any, 0, len(value))
		for _, inner := range value {
			out = append(out, sanitizeACPToolRawOutput(inner))
		}
		return out
	default:
		return raw
	}
}

func sanitizeACPContent(content []any) []any {
	out := make([]any, 0, len(content))
	for _, raw := range content {
		item, _ := raw.(map[string]any)
		if item == nil {
			out = append(out, raw)
			continue
		}
		if strings.TrimSpace(stringOrDefault(item["type"], "")) == "image" {
			image := clonePayload(item)
			if data := strings.TrimSpace(stringOrDefault(image["data"], "")); data != "" {
				image["data"] = fmt.Sprintf("<omitted base64 image data: %d chars>", len(data))
			}
			out = append(out, image)
			continue
		}
		out = append(out, sanitizeACPToolRawOutput(item))
	}
	return out
}
