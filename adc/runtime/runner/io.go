package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (r *Runner) persistActionEvent(
	turnIndex int,
	stepIndex int,
	actorRole string,
	actionType string,
	payload map[string]any,
	res map[string]any,
) error {
	if err := r.store.AppendEvent(r.cfg.RunID, turnIndex, stepIndex, actorRole, actionType, payload, res); err != nil {
		return err
	}
	if r.cfg.EventsPath == "" {
		return nil
	}
	line := map[string]any{
		"timestamp": time.Now().Format("2006-01-02 15:04:05.000"),
		"turn":      turnIndex,
		"step":      stepIndex,
		"role":      actorRole,
		"action":    actionType,
		"payload":   payload,
		"response":  res,
	}
	return appendEventLine(r.cfg.EventsPath, line)
}

func (r *Runner) persistAgentEvent(
	turnIndex int,
	sequence int,
	actorRole string,
	eventType string,
	payload map[string]any,
) error {
	stepIndex := -sequence
	if err := r.store.AppendEvent(r.cfg.RunID, turnIndex, stepIndex, actorRole, eventType, payload, map[string]any{}); err != nil {
		return err
	}
	if r.cfg.EventsPath == "" {
		return nil
	}
	line := map[string]any{
		"timestamp":   time.Now().Format("2006-01-02 15:04:05.000"),
		"turn":        turnIndex,
		"step":        stepIndex,
		"role":        actorRole,
		"agent_event": eventType,
		"payload":     payload,
	}
	return appendEventLine(r.cfg.EventsPath, line)
}

func (r *Runner) persistAgentCompletionResult(
	turnIndex int,
	sequence int,
	actorRole string,
	payload map[string]any,
) error {
	return r.persistAgentEvent(turnIndex, sequence, actorRole, "agent_completion_result", payload)
}

func appendEventLine(path string, line map[string]any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events path: %w", err)
	}
	enc, err := json.Marshal(line)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("marshal events line: %w", err)
	}
	if _, err := f.Write(append(enc, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write events path: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close events path: %w", err)
	}
	return nil
}

func resetEventLog(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create events directory: %w", err)
		}
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return fmt.Errorf("reset events path: %w", err)
	}
	return nil
}

func (r *Runner) writeEvidence(result Result) error {
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	if r.cfg.OutputPath == "" {
		return nil
	}
	if err := os.WriteFile(r.cfg.OutputPath, raw, 0o644); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}
	if err := exportACPWorkProduct(filepath.Dir(r.cfg.OutputPath), r.workProductDirs); err != nil {
		return err
	}
	return nil
}

func exportACPWorkProduct(outputDir string, workProductDirs map[string]string) error {
	if len(workProductDirs) == 0 {
		return nil
	}
	workRoot := filepath.Join(outputDir, "work-product")
	roles := make([]string, 0, len(workProductDirs))
	for role := range workProductDirs {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		src := strings.TrimSpace(workProductDirs[role])
		if src == "" {
			continue
		}
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat work-product dir for %s: %w", role, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("work-product path for %s is not a directory", role)
		}
		dst := filepath.Join(workRoot, role)
		if err := copyTree(dst, src); err != nil {
			return fmt.Errorf("export work product for %s: %w", role, err)
		}
	}
	return nil
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
