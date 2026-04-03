package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"adjudication/adc/runtime/courts"
	"adjudication/adc/runtime/lean"
	"adjudication/adc/runtime/spec"
	"adjudication/adc/runtime/store"
	"adjudication/common/openai"
)

type Config struct {
	ScenarioPath      string
	ScenarioBaseDir   string
	OutputPath        string
	EventsPath        string
	RunID             string
	Model             string
	Temperature       *float64
	JurorTemperature  *float64
	JurorPersonasPath string
	FlashJurorModel   string
	Offline           bool
	Runtime           RuntimeLimits
	ACP               *ACPConfig
}

type TurnLog struct {
	Source             string           `json:"source,omitempty"`
	ActionID           string           `json:"action_id,omitempty"`
	OpportunityID      string           `json:"opportunity_id,omitempty"`
	OpportunityPhase   string           `json:"opportunity_phase,omitempty"`
	OpportunityKind    string           `json:"opportunity_kind,omitempty"`
	OpportunityMessage string           `json:"opportunity_message,omitempty"`
	MayPass            bool             `json:"may_pass,omitempty"`
	Role               string           `json:"role"`
	Prompt             string           `json:"prompt"`
	Steps              int              `json:"steps"`
	Transcript         []map[string]any `json:"transcript"`
}

type Result struct {
	Scenario   string           `json:"scenario"`
	Assertions []map[string]any `json:"assertions"`
	TurnLogs   []TurnLog        `json:"turn_logs"`
	FinalState map[string]any   `json:"final_state"`
}

type ActionExecution struct {
	Result             map[string]any
	FollowupInputItems []map[string]any
}

type Runner struct {
	scenario                spec.FormalScenario
	lean                    lean.Engine
	store                   *store.Store
	client                  *openai.Client
	jurorClient             *openai.Client
	cfg                     Config
	state                   map[string]any
	roles                   map[string]spec.RoleSpec
	courtProfile            courts.Profile
	acpSessions             map[string]*acpPersistentSession
	jurorPersonaPool        *jurorPersonaPool
	jurorPersonaAssignments map[string]jurorPersonaPair
}

func (r *Runner) RequiresLLMTurns() bool {
	if r.scenario.LoopPolicy != nil && strings.TrimSpace(r.scenario.LoopPolicy.Type) == "autopilot_trial" {
		return true
	}
	for _, turn := range r.scenario.Turns {
		if turn.DeterministicAction == nil {
			return true
		}
	}
	return false
}

func New(st *store.Store, le lean.Engine, client *openai.Client, jurorClient *openai.Client, cfg Config) (*Runner, error) {
	scenario, err := spec.Load(cfg.ScenarioPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ScenarioBaseDir) == "" {
		cfg.ScenarioBaseDir = filepath.Dir(cfg.ScenarioPath)
	}
	cfg.Runtime = cfg.Runtime.Normalized()
	roles := make(map[string]spec.RoleSpec, len(scenario.Roles))
	for _, r := range scenario.Roles {
		roles[r.Name] = r
	}
	if err := loadRolePromptPreambles(roles, cfg.ScenarioBaseDir); err != nil {
		return nil, err
	}
	courtProfile, err := resolveScenarioCourtProfile(scenario)
	if err != nil {
		return nil, err
	}
	r := &Runner{
		scenario:                scenario,
		lean:                    le,
		store:                   st,
		client:                  client,
		jurorClient:             jurorClient,
		cfg:                     cfg,
		roles:                   roles,
		courtProfile:            courtProfile,
		acpSessions:             map[string]*acpPersistentSession{},
		jurorPersonaAssignments: map[string]jurorPersonaPair{},
	}
	if strings.TrimSpace(cfg.JurorPersonasPath) != "" {
		pool, err := loadJurorPersonaPool(cfg.JurorPersonasPath, cfg.ScenarioBaseDir, cfg.FlashJurorModel)
		if err != nil {
			return nil, err
		}
		r.jurorPersonaPool = pool
	}
	if err := validateScenarioActions(scenario, roles); err != nil {
		return nil, err
	}
	r.state = buildInitialState(scenario, courtProfile)
	if scenario.CaseInit != nil {
		state, err := initializeSeededCase(le, r.state, *scenario.CaseInit)
		if err != nil {
			return nil, err
		}
		r.state = state
	}
	return r, nil
}

func resolveScenarioCourtProfile(scenario spec.FormalScenario) (courts.Profile, error) {
	if scenario.Court != nil {
		return *scenario.Court, nil
	}
	return courts.Resolve(strings.TrimSpace(scenario.CourtName))
}

func initializeSeededCase(le lean.Engine, state map[string]any, init spec.CaseInitializationSpec) (map[string]any, error) {
	attachments := make([]map[string]any, 0, len(init.Attachments))
	for _, attachment := range init.Attachments {
		attachments = append(attachments, map[string]any{
			"file_id":         attachment.FileID,
			"label":           attachment.Label,
			"original_name":   attachment.OriginalName,
			"storage_relpath": attachment.StorageRelPath,
			"sha256":          attachment.Sha256,
			"size_bytes":      attachment.SizeBytes,
		})
	}
	jurisdictionalAllegations := map[string]any{}
	addString := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			jurisdictionalAllegations[key] = strings.TrimSpace(value)
		}
	}
	addString("jurisdiction_basis", init.JurisdictionBasis)
	addString("jurisdictional_statement", init.JurisdictionalStatement)
	addString("injury_statement", init.InjuryStatement)
	addString("causation_statement", init.CausationStatement)
	addString("redressability_statement", init.RedressabilityStatement)
	addString("ripeness_statement", init.RipenessStatement)
	addString("live_controversy_statement", init.LiveControversyStatement)
	addString("plaintiff_citizenship", init.PlaintiffCitizenship)
	addString("defendant_citizenship", init.DefendantCitizenship)
	addString("amount_in_controversy", init.AmountInControversy)
	resp, err := le.InitializeCase(
		state,
		strings.TrimSpace(init.ComplaintSummary),
		strings.TrimSpace(init.FiledBy),
		strings.TrimSpace(init.JuryDemandedOn),
		jurisdictionalAllegations,
		attachments,
	)
	if err != nil {
		return nil, fmt.Errorf("lean initialize_case failed: %w", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		raw, _ := json.Marshal(resp)
		return nil, fmt.Errorf("lean initialize_case error: %s", string(raw))
	}
	nextState, _ := resp["state"].(map[string]any)
	if nextState == nil {
		return nil, fmt.Errorf("lean initialize_case missing state")
	}
	return nextState, nil
}

func loadRolePromptPreambles(roles map[string]spec.RoleSpec, scenarioBaseDir string) error {
	for roleName, role := range roles {
		path := strings.TrimSpace(role.PromptPreambleFile)
		if path == "" {
			continue
		}
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(scenarioBaseDir, path)
		}
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("read prompt preamble file role=%s path=%s: %w", roleName, path, err)
		}
		fileText := strings.TrimSpace(string(raw))
		if fileText == "" {
			continue
		}
		if strings.TrimSpace(role.PromptPreamble) == "" {
			role.PromptPreamble = fileText
		} else {
			role.PromptPreamble = strings.TrimSpace(role.PromptPreamble) + "\n\n" + fileText
		}
		roles[roleName] = role
	}
	return nil
}

func validateScenarioActions(scenario spec.FormalScenario, roles map[string]spec.RoleSpec) error {
	missing := make([]string, 0)
	seen := map[string]bool{}
	for _, role := range scenario.Roles {
		for _, action := range role.EffectiveAllowedActions() {
			action = strings.TrimSpace(action)
			if action == "" || seen[action] || toolSchema(action) != nil {
				continue
			}
			missing = append(missing, action)
			seen[action] = true
		}
	}
	for i, turn := range scenario.Turns {
		role, ok := roles[turn.Role]
		if !ok {
			return fmt.Errorf("turn %d uses unknown role: %s", i+1, turn.Role)
		}
		if turn.DeterministicAction != nil {
			action := strings.TrimSpace(turn.DeterministicAction.ActionType)
			if action == "" {
				return fmt.Errorf("turn %d deterministic action missing action_type", i+1)
			}
			if toolSchema(action) == nil && !seen[action] {
				missing = append(missing, action)
				seen[action] = true
			}
			continue
		}
		for _, action := range turn.EffectiveAllowedActions(role) {
			if toolSchema(action) != nil || seen[action] {
				continue
			}
			missing = append(missing, action)
			seen[action] = true
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"scenario contains actions not implemented in go runner: %s",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	if err := resetEventLog(r.cfg.EventsPath); err != nil {
		return Result{}, err
	}
	if err := r.store.CreateRun(r.cfg.RunID, r.scenario.Name); err != nil {
		return Result{}, err
	}
	turnLogs := make([]TurnLog, 0, len(r.scenario.Turns)+16)
	for i, turn := range r.scenario.Turns {
		role, ok := r.roles[turn.Role]
		if !ok {
			return Result{}, fmt.Errorf("unknown role: %s", turn.Role)
		}
		allowed := turn.EffectiveAllowedActions(role)
		if len(allowed) == 0 {
			return Result{}, fmt.Errorf("turn role=%s has no allowed actions", turn.Role)
		}
		log, err := r.executeTurn(ctx, i+1, role, turn, allowed)
		if err != nil {
			if cleanupErr := r.closeACPSessions(); cleanupErr != nil {
				return Result{}, errors.Join(err, cleanupErr)
			}
			return Result{}, err
		}
		turnLogs = append(turnLogs, log)
	}
	if r.scenario.LoopPolicy != nil && strings.TrimSpace(r.scenario.LoopPolicy.Type) == "autopilot_trial" {
		logs, err := r.runAutopilot(ctx, len(turnLogs)+1)
		if err != nil {
			if cleanupErr := r.closeACPSessions(); cleanupErr != nil {
				return Result{}, errors.Join(err, cleanupErr)
			}
			return Result{}, err
		}
		turnLogs = append(turnLogs, logs...)
	}
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

func (r *Runner) executeAction(turnIndex, stepIndex int, actorRole, actionType string, payload map[string]any) (ActionExecution, error) {
	preparedPayload, err := r.prepareActionPayload(actionType, payload)
	if err != nil {
		return ActionExecution{}, err
	}
	payload = preparedPayload
	execRes, handled, err := r.executeLocalAction(actorRole, actionType, payload)
	if err != nil {
		return ActionExecution{}, err
	}
	if !handled {
		res, err := r.lean.Step(r.state, actionType, actorRole, payload)
		if err != nil {
			return ActionExecution{}, err
		}
		execRes = ActionExecution{Result: res}
	}
	res := execRes.Result
	if ok, _ := res["ok"].(bool); ok {
		if state, ok := res["state"].(map[string]any); ok {
			if handled {
				r.state = state
			} else {
				r.state = mergeLocalCaseExtensions(r.state, state)
			}
		}
	}
	if err := r.persistActionEvent(turnIndex, stepIndex, actorRole, actionType, payload, res); err != nil {
		return ActionExecution{}, err
	}
	return execRes, nil
}
