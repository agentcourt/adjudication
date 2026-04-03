package runner

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"adjudication/common/openai"
	"adjudication/common/persona"
	"adjudication/common/xproxy"
)

type jurorPersonaPair struct {
	Model       string
	PersonaText string
	PersonaFile string
}

type jurorPersonaPool struct {
	pairs     []jurorPersonaPair
	remaining []int
}

func (p *jurorPersonaPool) findPair(model string, personaFile string) (jurorPersonaPair, bool) {
	if p == nil {
		return jurorPersonaPair{}, false
	}
	model = strings.TrimSpace(model)
	personaFile = strings.TrimSpace(personaFile)
	for _, pair := range p.pairs {
		if strings.TrimSpace(pair.Model) == model && strings.TrimSpace(pair.PersonaFile) == personaFile {
			return pair, true
		}
	}
	return jurorPersonaPair{}, false
}

func loadJurorPersonaPool(path string, scenarioBaseDir string, flashModel string) (*jurorPersonaPool, error) {
	resolvedPairsPath := resolveScenarioRelativePath(path, scenarioBaseDir)
	raw, err := os.ReadFile(resolvedPairsPath)
	if err != nil {
		return nil, fmt.Errorf("read juror personas file: %w", err)
	}
	flashModel = strings.TrimSpace(flashModel)
	lines := strings.Split(string(raw), "\n")
	pairs := make([]jurorPersonaPair, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		spec, err := persona.ParseRecord(line, filepath.Dir(resolvedPairsPath))
		if err != nil {
			return nil, err
		}
		if flashModel != "" {
			spec.Model = flashModel
		}
		pairs = append(pairs, jurorPersonaPair{
			Model:       spec.Model,
			PersonaText: spec.Text,
			PersonaFile: spec.File,
		})
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("juror personas file contains no usable pairs: %s", path)
	}
	remaining := make([]int, len(pairs))
	for i := range pairs {
		remaining[i] = i
	}
	return &jurorPersonaPool{pairs: pairs, remaining: remaining}, nil
}

func (p *jurorPersonaPool) samplePair() (jurorPersonaPair, error) {
	if p == nil || len(p.pairs) == 0 {
		return jurorPersonaPair{}, fmt.Errorf("juror persona pool is empty")
	}
	if len(p.remaining) == 0 {
		return jurorPersonaPair{}, fmt.Errorf("juror persona pool exhausted; add more records")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(p.remaining))))
	if err != nil {
		return jurorPersonaPair{}, fmt.Errorf("sample juror persona pair: %w", err)
	}
	remainingIndex := int(n.Int64())
	pairIndex := p.remaining[remainingIndex]
	p.remaining = append(p.remaining[:remainingIndex], p.remaining[remainingIndex+1:]...)
	return p.pairs[pairIndex], nil
}

func (r *Runner) prepareActionPayload(actionType string, payload map[string]any) (map[string]any, error) {
	if actionType != "add_juror" {
		return payload, nil
	}
	return r.applyJurorPersonaDefaults(payload)
}

func (r *Runner) applyJurorPersonaDefaults(payload map[string]any) (map[string]any, error) {
	cloned := clonePayload(payload)
	jurorID := strings.TrimSpace(stringOrDefault(cloned["juror_id"], ""))
	if jurorID == "" {
		return cloned, nil
	}
	if pair, ok := r.jurorPersonaAssignments[jurorID]; ok {
		if strings.TrimSpace(stringOrDefault(cloned["model"], "")) == "" && strings.TrimSpace(pair.Model) != "" {
			cloned["model"] = pair.Model
		}
		if strings.TrimSpace(stringOrDefault(cloned["persona_filename"], "")) == "" && strings.TrimSpace(pair.PersonaFile) != "" {
			cloned["persona_filename"] = pair.PersonaFile
		}
		return cloned, nil
	}
	if r.jurorPersonaPool == nil {
		return cloned, nil
	}
	hasModel := strings.TrimSpace(stringOrDefault(cloned["model"], "")) != ""
	hasPersona := strings.TrimSpace(stringOrDefault(cloned["persona_filename"], "")) != ""
	if hasModel && hasPersona {
		if pair, ok := r.jurorPersonaPool.findPair(stringOrDefault(cloned["model"], ""), stringOrDefault(cloned["persona_filename"], "")); ok {
			r.jurorPersonaAssignments[jurorID] = pair
		}
		return cloned, nil
	}
	pair, err := r.jurorPersonaPool.samplePair()
	if err != nil {
		return nil, err
	}
	cloned["model"] = pair.Model
	cloned["persona_filename"] = pair.PersonaFile
	r.jurorPersonaAssignments[jurorID] = pair
	return cloned, nil
}

var jurorIDPattern = regexp.MustCompile(`\bJ(\d+)\b`)

func opportunityConstraintString(opportunity leanOpportunity, key string) string {
	required, _ := opportunity.Constraints["required_payload"].(map[string]any)
	if required != nil {
		if value := strings.TrimSpace(stringOrDefault(required[key], "")); value != "" {
			return value
		}
	}
	defaults, _ := opportunity.Constraints["payload_defaults"].(map[string]any)
	if defaults != nil {
		if value := strings.TrimSpace(stringOrDefault(defaults[key], "")); value != "" {
			return value
		}
	}
	return ""
}

func countCandidateJurors(state map[string]any) int {
	caseObj, _ := state["case"].(map[string]any)
	if caseObj == nil {
		return 0
	}
	jurors, _ := caseObj["jurors"].([]any)
	count := 0
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if juror == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(juror["status"], "")) == "candidate" {
			count++
		}
	}
	return count
}

func nextJurorNumber(state map[string]any) int {
	caseObj, _ := state["case"].(map[string]any)
	if caseObj == nil {
		return 1
	}
	jurors, _ := caseObj["jurors"].([]any)
	maxSeen := 0
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if juror == nil {
			continue
		}
		jurorID := strings.TrimSpace(stringOrDefault(juror["juror_id"], ""))
		match := jurorIDPattern.FindStringSubmatch(jurorID)
		if len(match) != 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if n > maxSeen {
			maxSeen = n
		}
	}
	if maxSeen == 0 {
		return 1
	}
	return maxSeen + 1
}

func targetJurorIDForOpportunity(opportunity leanOpportunity) string {
	if jurorID := opportunityConstraintString(opportunity, "juror_id"); jurorID != "" {
		return jurorID
	}
	match := jurorIDPattern.FindStringSubmatch(opportunity.ActorMessage)
	if len(match) != 2 {
		return ""
	}
	return "J" + match[1]
}

func (r *Runner) jurorOpportunityPromptContext(opportunity leanOpportunity) (string, string) {
	jurorID := targetJurorIDForOpportunity(opportunity)
	if jurorID == "" {
		return "", ""
	}
	pair, ok := r.jurorPersonaAssignments[jurorID]
	if !ok {
		return "", ""
	}
	model := strings.TrimSpace(pair.Model)
	context := strings.TrimSpace(pair.PersonaText)
	if context == "" {
		return model, ""
	}
	return model, persona.JurorPrompt(jurorID, context)
}

func (r *Runner) jurorResponseClient(model string) (*openai.Client, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		if r.client == nil {
			return nil, fmt.Errorf("llm client is nil")
		}
		return r.client, nil
	}
	if _, err := xproxy.ParseXProxyModel(model); err != nil {
		return nil, fmt.Errorf("invalid juror model %q: %w", model, err)
	}
	if r.jurorClient == nil {
		return nil, fmt.Errorf("juror xproxy client is nil for model %q", model)
	}
	return r.jurorClient, nil
}
