package xproxy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func llmCSVTimestamp(ts time.Time) string {
	return ts.Format("2006-01-02 15:04:05.000")
}

func llmCSVLineAt(ts time.Time, model string, bytesIn int, bytesOut int, elapsedMillis int64) string {
	return fmt.Sprintf("llm_csv,%s,%s,%d,%d,%d", llmCSVTimestamp(ts), model, bytesIn, bytesOut, elapsedMillis)
}

func llmCSVLine(model string, bytesIn int, bytesOut int, elapsedMillis int64) string {
	return llmCSVLineAt(time.Now(), model, bytesIn, bytesOut, elapsedMillis)
}

func logLLMCSV(model string, bytesIn int, bytesOut int, elapsedMillis int64) {
	logf("%s", llmCSVLine(model, bytesIn, bytesOut, elapsedMillis))
}

func llmCSVModel(spec ModelSpec) string {
	model := spec.Endpoint + "://" + spec.ModelIn
	if spec.ForceSearch {
		return model + "?tools=search"
	}
	return model
}

func sortedKeys[K ~string, V any](m map[K]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)
	return keys
}

func sortStrings(values []string) {
	sort.Strings(values)
}

func joinBullets(values []string) string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = "- `" + value + "`"
	}
	return strings.Join(out, "\n")
}

func stringValue(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case json.Number:
		return s.String()
	case fmt.Stringer:
		return s.String()
	default:
		return ""
	}
}

func boolValue(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		parsed, err := strconv.ParseBool(b)
		return err == nil && parsed
	default:
		return false
	}
}

func typeName(v any) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

func truncateASCII(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func optionalInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func formatOptionalInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func randomHex(size int) string {
	buf := make([]byte, size/2)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func filepathDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}

func appendJSONL(path string, payload any) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(payload)
}

func appendLog(logLines *[]string, msg string) {
	if logLines != nil {
		*logLines = append(*logLines, msg)
		return
	}
	logf("%s", msg)
}

func exitCode(state *os.ProcessState) int {
	if state == nil {
		return 1
	}
	return state.ExitCode()
}

func exitCodeFromWait(waitErr error, state *os.ProcessState) int {
	if waitErr == nil {
		return exitCode(state)
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func shQuoteCommand(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = strconv.Quote(arg)
		quoted[i] = strings.TrimPrefix(strings.TrimSuffix(quoted[i], `"`), `"`)
		if strings.ContainsAny(arg, " \t\n\"'`$\\") {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		} else {
			quoted[i] = arg
		}
	}
	return strings.Join(quoted, " ")
}
