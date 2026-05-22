package runner

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const artifactManifestSchemaVersion = "aar.artifact-manifest.v0"

type artifactReadBudget struct {
	bytes int
	reads int
}

func utcTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func artifactIDForFile(sha string, artifactID string) string {
	stem := strings.TrimSuffix(filepath.ToSlash(artifactID), filepath.Ext(artifactID))
	stem = strings.ToLower(strings.TrimSpace(stem))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		slug = "artifact"
	}
	return fmt.Sprintf("art_%s_%s", sha[:12], slug)
}

func canonicalArtifactID(sha string, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if strings.HasPrefix(candidate, "art_"+sha[:12]+"_") {
		return candidate
	}
	return artifactIDForFile(sha, candidate)
}

func copyToArtifactStore(path string, storeDir string, sha string) (string, error) {
	storageName := filepath.ToSlash(filepath.Join(sha[:2], sha))
	dst := filepath.Join(storeDir, filepath.FromSlash(storageName))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("create artifact store dir: %w", err)
	}
	if _, err := os.Stat(dst); err == nil {
		existing, err := sha256File(dst)
		if err != nil {
			return "", err
		}
		if existing != sha {
			return "", fmt.Errorf("artifact store collision at %s", dst)
		}
		return storageName, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat artifact store file %s: %w", dst, err)
	}
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact source %s: %w", path, err)
	}
	defer src.Close()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create artifact store file %s: %w", dst, err)
	}
	_, copyErr := io.Copy(dstFile, src)
	closeErr := dstFile.Close()
	if copyErr != nil {
		return "", fmt.Errorf("copy artifact bytes to %s: %w", dst, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close artifact store file %s: %w", dst, closeErr)
	}
	return storageName, nil
}

func (rc *runContext) initializeArtifactRegistry() error {
	rc.artifactStoreDir = filepath.Join(rc.cfg.OutputDir, "artifact-store")
	rc.artifactByID = map[string]ArtifactMeta{}
	rc.artifacts = []ArtifactMeta{}
	fileByID := map[string]CaseFile{}
	for i, file := range rc.caseFiles {
		artifact, err := rc.registerCaseFileArtifact(file)
		if err != nil {
			return err
		}
		file.ArtifactID = artifact.ArtifactID
		rc.caseFiles[i] = file
		fileByID[file.ArtifactID] = file
	}
	rc.fileByID = fileByID
	return nil
}

func (rc *runContext) registerCaseFileArtifact(file CaseFile) (ArtifactMeta, error) {
	meta, err := rc.buildArtifactMeta(file.Path, ArtifactMeta{
		ArtifactID:          file.ArtifactID,
		Title:               file.ArtifactID,
		OriginalName:        file.Name,
		MimeType:            file.MimeType,
		SourceDescription:   "Initial case packet file",
		SubmittedByRole:     "system",
		SubmittedPhase:      "case_packet",
		AdmissibilityStatus: "case_packet",
		RecordVisibility:    "juror_visible",
		TextReadable:        file.TextReadable,
	})
	if err != nil {
		return ArtifactMeta{}, err
	}
	return rc.addArtifact(meta)
}

func (rc *runContext) registerSubmittedArtifactArtifact(meta SubmittedArtifactMeta, file CaseFile) (ArtifactMeta, error) {
	artifact, err := rc.buildArtifactMeta(file.Path, ArtifactMeta{
		ArtifactID:          meta.ArtifactID,
		Title:               meta.Title,
		OriginalName:        meta.Name,
		MimeType:            meta.MimeType,
		SourceURL:           meta.SourceURL,
		SourceDescription:   meta.SourceDescription,
		RetrievalTimestamp:  meta.RetrievalTimestamp,
		SubmittedByRole:     meta.Role,
		SubmittedPhase:      meta.Phase,
		AdmissibilityStatus: "submitted_artifacts",
		RecordVisibility:    "juror_visible",
		Relevance:           meta.Relevance,
		TextReadable:        file.TextReadable,
	})
	if err != nil {
		return ArtifactMeta{}, err
	}
	return rc.addArtifact(artifact)
}

func (rc *runContext) buildArtifactMeta(path string, meta ArtifactMeta) (ArtifactMeta, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ArtifactMeta{}, fmt.Errorf("stat artifact source %s: %w", path, err)
	}
	if info.IsDir() {
		return ArtifactMeta{}, fmt.Errorf("artifact source is a directory: %s", path)
	}
	sha, err := sha256File(path)
	if err != nil {
		return ArtifactMeta{}, err
	}
	storageName, err := copyToArtifactStore(path, rc.artifactStoreDir, sha)
	if err != nil {
		return ArtifactMeta{}, err
	}
	meta.ArtifactID = canonicalArtifactID(sha, meta.ArtifactID)
	meta.SHA256 = sha
	meta.SizeBytes = int(info.Size())
	if strings.TrimSpace(meta.MimeType) == "" {
		meta.MimeType = "application/octet-stream"
	}
	meta.StorageName = storageName
	meta.CreatedAt = utcTimestamp()
	return meta, nil
}

func (rc *runContext) addArtifact(meta ArtifactMeta) (ArtifactMeta, error) {
	if meta.ArtifactID == "" {
		return ArtifactMeta{}, fmt.Errorf("artifact_id is required")
	}
	if rc.artifactByID == nil {
		rc.artifactByID = map[string]ArtifactMeta{}
	}
	if existing, ok := rc.artifactByID[meta.ArtifactID]; ok {
		if conflict := artifactMetadataConflict(existing, meta); conflict != "" {
			return ArtifactMeta{}, fmt.Errorf("artifact_id collision %s: %s", meta.ArtifactID, conflict)
		}
		return existing, nil
	}
	rc.artifacts = append(rc.artifacts, meta)
	rc.artifactByID[meta.ArtifactID] = meta
	return meta, nil
}

func artifactMetadataConflict(existing ArtifactMeta, incoming ArtifactMeta) string {
	left := existing
	right := incoming
	left.CreatedAt = ""
	right.CreatedAt = ""
	if left == right {
		return ""
	}
	if existing.SHA256 != incoming.SHA256 || existing.SizeBytes != incoming.SizeBytes || existing.StorageName != incoming.StorageName {
		return fmt.Sprintf("existing sha=%s size=%d storage=%s, incoming sha=%s size=%d storage=%s", existing.SHA256, existing.SizeBytes, existing.StorageName, incoming.SHA256, incoming.SizeBytes, incoming.StorageName)
	}
	return "metadata differs for existing artifact_id"
}

func (rc *runContext) artifactPath(meta ArtifactMeta) (string, error) {
	storageName := strings.TrimSpace(meta.StorageName)
	if storageName == "" {
		return "", fmt.Errorf("artifact %s has no storage_name", meta.ArtifactID)
	}
	if filepath.IsAbs(storageName) {
		return storageName, nil
	}
	return filepath.Join(rc.artifactStoreDir, filepath.FromSlash(storageName)), nil
}

func (rc *runContext) artifactManifest() map[string]any {
	artifacts := append([]ArtifactMeta(nil), rc.artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].ArtifactID < artifacts[j].ArtifactID })
	return map[string]any{
		"schema_version": artifactManifestSchemaVersion,
		"created_at":     utcTimestamp(),
		"artifact_count": len(artifacts),
		"artifacts":      artifacts,
	}
}

func (rc *runContext) listVisibleArtifacts() []ArtifactMeta {
	out := make([]ArtifactMeta, 0, len(rc.artifacts))
	for _, artifact := range rc.artifacts {
		if artifact.RecordVisibility == "system_private" {
			continue
		}
		out = append(out, artifact)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ArtifactID < out[j].ArtifactID })
	return out
}

func (rc *runContext) statArtifact(artifactID string) (ArtifactMeta, error) {
	artifactID = strings.TrimSpace(artifactID)
	if artifactID == "" {
		return ArtifactMeta{}, fmt.Errorf("artifact_id is required")
	}
	meta, ok := rc.artifactByID[artifactID]
	if !ok || meta.RecordVisibility == "system_private" {
		return ArtifactMeta{}, fmt.Errorf("unknown artifact %q", artifactID)
	}
	path, err := rc.artifactPath(meta)
	if err != nil {
		return ArtifactMeta{}, err
	}
	sha, err := sha256File(path)
	if err != nil {
		return ArtifactMeta{}, err
	}
	if sha != meta.SHA256 {
		return ArtifactMeta{}, fmt.Errorf("artifact %s sha256 mismatch", artifactID)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ArtifactMeta{}, fmt.Errorf("stat artifact %s: %w", artifactID, err)
	}
	if int(info.Size()) != meta.SizeBytes {
		return ArtifactMeta{}, fmt.Errorf("artifact %s size mismatch", artifactID)
	}
	return meta, nil
}

func (rc *runContext) readArtifactRange(artifactID string, offset int64, length int, budget *artifactReadBudget) (map[string]any, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}
	if length > rc.cfg.Policy.MaxArtifactReadBytes {
		return nil, fmt.Errorf("artifact_read_limit_exceeded: requested %d, max %d", length, rc.cfg.Policy.MaxArtifactReadBytes)
	}
	if budget != nil {
		if budget.reads >= rc.cfg.Policy.MaxArtifactReadsPerOpportunity {
			return nil, fmt.Errorf("artifact_read_limit_exceeded: read count limit %d", rc.cfg.Policy.MaxArtifactReadsPerOpportunity)
		}
		if budget.bytes+length > rc.cfg.Policy.MaxArtifactReadBytesPerOpportunity {
			return nil, fmt.Errorf("artifact_read_limit_exceeded: opportunity byte budget %d", rc.cfg.Policy.MaxArtifactReadBytesPerOpportunity)
		}
	}
	meta, err := rc.statArtifact(artifactID)
	if err != nil {
		return nil, err
	}
	path, err := rc.artifactPath(meta)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open artifact %s: %w", artifactID, err)
	}
	defer f.Close()
	if offset > int64(meta.SizeBytes) {
		return nil, fmt.Errorf("invalid_artifact_range: offset %d exceeds size %d", offset, meta.SizeBytes)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek artifact %s: %w", artifactID, err)
	}
	raw := make([]byte, length)
	n, err := f.Read(raw)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read artifact %s: %w", artifactID, err)
	}
	raw = raw[:n]
	if budget != nil {
		budget.reads++
		budget.bytes += n
	}
	return map[string]any{
		"artifact_id":      meta.ArtifactID,
		"offset":           offset,
		"length":           n,
		"total_size_bytes": meta.SizeBytes,
		"sha256":           meta.SHA256,
		"mime_type":        meta.MimeType,
		"content_base64":   base64.StdEncoding.EncodeToString(raw),
	}, nil
}

func safeMaterializedArtifactName(meta ArtifactMeta) string {
	name := filepath.Base(strings.TrimSpace(meta.OriginalName))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = meta.ArtifactID
	}
	return meta.ArtifactID + "-" + name
}

func (rc *runContext) materializeArtifact(workspaceDir string, artifactID string) (map[string]any, error) {
	meta, err := rc.statArtifact(artifactID)
	if err != nil {
		return nil, err
	}
	path, err := rc.artifactPath(meta)
	if err != nil {
		return nil, err
	}
	artifactsDir := filepath.Join(workspaceDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifacts workspace dir: %w", err)
	}
	dst := filepath.Join(artifactsDir, safeMaterializedArtifactName(meta))
	if _, err := os.Stat(dst); err == nil {
		existing, err := sha256File(dst)
		if err != nil {
			return nil, err
		}
		if existing != meta.SHA256 {
			return nil, fmt.Errorf("materialization target exists with different bytes: %s", dst)
		}
	} else if os.IsNotExist(err) {
		if err := copyFile(dst, path); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("stat materialized artifact %s: %w", dst, err)
	}
	remote := filepath.ToSlash(filepath.Join(defaultRemoteSessionCwd, "artifacts", filepath.Base(dst)))
	return map[string]any{
		"artifact_id":    meta.ArtifactID,
		"workspace_path": remote,
		"sha256":         meta.SHA256,
		"size_bytes":     meta.SizeBytes,
		"mime_type":      meta.MimeType,
	}, nil
}

func randomUploadID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return "upl_" + hex.EncodeToString(buf), nil
}

func (rc *runContext) beginArtifactUpload(opportunity Opportunity, params map[string]any) (*ArtifactUploadSession, error) {
	if opportunity.Phase != "arguments" && opportunity.Phase != "rebuttals" {
		return nil, fmt.Errorf("artifact upload is allowed only in arguments and rebuttals")
	}
	title := mapString(params["title"])
	mimeType := mapString(params["mime_type"])
	relevance := mapString(params["relevance"])
	sourceURL := mapString(params["source_url"])
	sourceDescription := mapString(params["source_description"])
	expectedSize, err := requiredIntParam(params, "expected_size_bytes")
	if err != nil {
		return nil, err
	}
	if title == "" {
		return nil, fmt.Errorf("artifact upload requires title")
	}
	if sourceURL == "" && sourceDescription == "" {
		return nil, fmt.Errorf("artifact upload requires source_url or source_description")
	}
	if mimeType == "" {
		return nil, fmt.Errorf("artifact upload requires mime_type")
	}
	if relevance == "" {
		return nil, fmt.Errorf("artifact upload requires relevance")
	}
	if expectedSize <= 0 {
		return nil, fmt.Errorf("artifact upload requires positive expected_size_bytes")
	}
	if expectedSize > rc.cfg.Policy.MaxArtifactUploadBytes {
		return nil, fmt.Errorf("artifact upload exceeds byte limit of %d", rc.cfg.Policy.MaxArtifactUploadBytes)
	}
	if submittedArtifactCountForRole(rc.submittedArtifact, opportunity.Role) >= rc.cfg.Policy.MaxSubmittedArtifactPerSide {
		return nil, fmt.Errorf("submitted_artifacts for this side exceed limit of %d", rc.cfg.Policy.MaxSubmittedArtifactPerSide)
	}
	uploadID, err := randomUploadID()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(rc.cfg.OutputDir, "artifact-uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact upload dir: %w", err)
	}
	session := &ArtifactUploadSession{
		UploadID:           uploadID,
		Role:               opportunity.Role,
		Phase:              opportunity.Phase,
		Title:              title,
		MimeType:           mimeType,
		ExpectedSizeBytes:  expectedSize,
		ExpectedSHA256:     strings.ToLower(mapString(params["expected_sha256"])),
		SourceURL:          sourceURL,
		SourceDescription:  sourceDescription,
		RetrievalTimestamp: mapString(params["retrieval_timestamp"]),
		Relevance:          relevance,
		ParentArtifactID:   mapString(params["parent_artifact_id"]),
		DerivationMethod:   mapString(params["derivation_method"]),
		Path:               filepath.Join(dir, uploadID+".part"),
	}
	if rc.uploadSessions == nil {
		rc.uploadSessions = map[string]*ArtifactUploadSession{}
	}
	rc.uploadSessions[uploadID] = session
	return session, nil
}

func (rc *runContext) writeArtifactChunk(uploadID string, offset int, contentBase64 string) (*ArtifactUploadSession, int, error) {
	session := rc.uploadSessions[strings.TrimSpace(uploadID)]
	if session == nil {
		return nil, 0, fmt.Errorf("unknown upload_id %q", uploadID)
	}
	if offset != session.ReceivedBytes {
		return nil, 0, fmt.Errorf("artifact upload offset %d does not match received byte count %d", offset, session.ReceivedBytes)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(contentBase64))
	if err != nil {
		return nil, 0, fmt.Errorf("decode artifact chunk: %w", err)
	}
	if len(raw) == 0 {
		return nil, 0, fmt.Errorf("artifact chunk must not be empty")
	}
	if len(raw) > rc.cfg.Policy.MaxArtifactChunkBytes {
		return nil, 0, fmt.Errorf("artifact chunk exceeds byte limit of %d", rc.cfg.Policy.MaxArtifactChunkBytes)
	}
	if session.ReceivedBytes+len(raw) > session.ExpectedSizeBytes {
		return nil, 0, fmt.Errorf("artifact upload exceeds expected size of %d", session.ExpectedSizeBytes)
	}
	f, err := os.OpenFile(session.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, 0, fmt.Errorf("open artifact upload %s: %w", session.UploadID, err)
	}
	written, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil {
		return nil, 0, fmt.Errorf("write artifact upload %s: %w", session.UploadID, writeErr)
	}
	if written != len(raw) {
		return nil, 0, fmt.Errorf("short artifact upload write %s: wrote %d of %d bytes", session.UploadID, written, len(raw))
	}
	if closeErr != nil {
		return nil, 0, fmt.Errorf("close artifact upload %s: %w", session.UploadID, closeErr)
	}
	session.ReceivedBytes += len(raw)
	return session, len(raw), nil
}

func (rc *runContext) prepareArtifactUploadCommit(session *ArtifactUploadSession, preferredExt string) (SubmittedArtifactMeta, error) {
	if session == nil {
		return SubmittedArtifactMeta{}, fmt.Errorf("upload session is required")
	}
	if session.ReceivedBytes != session.ExpectedSizeBytes {
		return SubmittedArtifactMeta{}, fmt.Errorf("artifact upload incomplete: received %d of %d bytes", session.ReceivedBytes, session.ExpectedSizeBytes)
	}
	sha, err := sha256File(session.Path)
	if err != nil {
		return SubmittedArtifactMeta{}, err
	}
	if session.ExpectedSHA256 != "" && session.ExpectedSHA256 != sha {
		return SubmittedArtifactMeta{}, fmt.Errorf("artifact upload sha256 mismatch: expected %s, got %s", session.ExpectedSHA256, sha)
	}
	name := submittedArtifactFilename(len(rc.submittedArtifact)+1, session.Role, sha, session.MimeType, preferredExt)
	return SubmittedArtifactMeta{
		Phase:              session.Phase,
		Role:               session.Role,
		ArtifactID:         artifactIDForFile(sha, name),
		Name:               name,
		Title:              session.Title,
		SourceURL:          session.SourceURL,
		SourceDescription:  session.SourceDescription,
		MimeType:           session.MimeType,
		RetrievalTimestamp: session.RetrievalTimestamp,
		Relevance:          session.Relevance,
		SHA256:             sha,
		SizeBytes:          session.ReceivedBytes,
	}, nil
}

func (rc *runContext) finalizeArtifactUpload(session *ArtifactUploadSession, meta SubmittedArtifactMeta) (SubmittedArtifactMeta, CaseFile, ArtifactMeta, error) {
	file, err := rc.moveUploadedSubmittedArtifactFile(meta, session.Path)
	if err != nil {
		return SubmittedArtifactMeta{}, CaseFile{}, ArtifactMeta{}, err
	}
	artifact, err := rc.registerSubmittedArtifactArtifact(meta, file)
	if err != nil {
		return SubmittedArtifactMeta{}, CaseFile{}, ArtifactMeta{}, err
	}
	artifact.ParentArtifactID = session.ParentArtifactID
	artifact.DerivationMethod = session.DerivationMethod
	if artifact.ParentArtifactID != "" || artifact.DerivationMethod != "" {
		if updated, err := rc.addArtifact(artifact); err == nil {
			artifact = updated
		}
	}
	meta.ArtifactID = artifact.ArtifactID
	delete(rc.uploadSessions, session.UploadID)
	return meta, file, artifact, nil
}

func (rc *runContext) moveUploadedSubmittedArtifactFile(meta SubmittedArtifactMeta, src string) (CaseFile, error) {
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
	if err := os.Rename(src, path); err != nil {
		return CaseFile{}, fmt.Errorf("move submitted evidence %s: %w", path, err)
	}
	_, readableKind := caseFileKind(name)
	textReadable := readableKind || strings.HasPrefix(strings.ToLower(meta.MimeType), "text/") || strings.EqualFold(meta.MimeType, "application/json")
	file := CaseFile{ArtifactID: meta.ArtifactID, Name: name, Path: path, MimeType: meta.MimeType, TextReadable: textReadable && meta.SizeBytes <= rc.cfg.Policy.MaxArtifactReadBytes, SizeBytes: meta.SizeBytes}
	if file.TextReadable {
		raw, err := os.ReadFile(path)
		if err != nil {
			return CaseFile{}, fmt.Errorf("read text uploaded evidence %s: %w", path, err)
		}
		file.Text = string(raw)
	}
	return file, nil
}

func copyFile(dst string, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", dst, closeErr)
	}
	return nil
}
