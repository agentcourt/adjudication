package proceeding

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type CasePacketOptions struct {
	ComplaintPath string
	CaseFiles     []string
	PacketPath    string
	ManifestPath  string
}

type CasePacketSummary struct {
	SchemaVersion string               `json:"schema_version"`
	CaseFileMode  string               `json:"case_file_mode"`
	Complaint     string               `json:"complaint"`
	CaseFiles     []string             `json:"case_files"`
	Files         []CasePacketFileMeta `json:"files"`
	Control       CasePacketControl    `json:"control"`
	PacketSHA384  string               `json:"packet_sha384,omitempty"`
	PacketBytes   int64                `json:"packet_bytes,omitempty"`
}

type CasePacketFileMeta struct {
	ArchivePath string `json:"archive_path"`
	SourceName  string `json:"source_name"`
	Role        string `json:"role"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA384      string `json:"sha384"`
}

type CasePacketControl struct {
	CaseArgs       string `json:"case_args"`
	CaseArgsSHA384 string `json:"case_args_sha384"`
}

type casePacketEntry struct {
	archivePath string
	sourcePath  string
	role        string
}

func WriteCasePacket(opts CasePacketOptions) (CasePacketSummary, error) {
	complaintPath := strings.TrimSpace(opts.ComplaintPath)
	packetPath := strings.TrimSpace(opts.PacketPath)
	manifestPath := strings.TrimSpace(opts.ManifestPath)
	if complaintPath == "" || packetPath == "" || manifestPath == "" {
		return CasePacketSummary{}, fmt.Errorf("complaint, packet, and manifest paths are required")
	}
	complaintInfo, err := requirePacketRegularFile(complaintPath, "complaint")
	if err != nil {
		return CasePacketSummary{}, err
	}
	complaintName := filepath.Base(complaintPath)
	if err := validateCasePacketName(complaintName, "complaint file name"); err != nil {
		return CasePacketSummary{}, err
	}
	complaintArchivePath := filepath.ToSlash(filepath.Join("case", complaintName))
	entries := []casePacketEntry{{archivePath: complaintArchivePath, sourcePath: complaintPath, role: "complaint"}}
	caseFileMode := "auto"
	explicitArchivePaths := []string{}
	if len(opts.CaseFiles) > 0 {
		caseFileMode = "explicit"
		resolved, err := resolveExplicitCaseFiles(opts.CaseFiles)
		if err != nil {
			return CasePacketSummary{}, err
		}
		if _, err := loadCaseFilesFromPaths(resolved); err != nil {
			return CasePacketSummary{}, err
		}
		for i, path := range resolved {
			name := filepath.Base(path)
			if err := validateCasePacketName(name, "case file name"); err != nil {
				return CasePacketSummary{}, err
			}
			archivePath := filepath.ToSlash(filepath.Join("case-files", fmt.Sprintf("%06d", i+1), name))
			entries = append(entries, casePacketEntry{archivePath: archivePath, sourcePath: path, role: "case_file"})
			explicitArchivePaths = append(explicitArchivePaths, archivePath)
		}
	} else {
		caseFiles, err := loadCaseFiles(filepath.Dir(complaintPath))
		if err != nil {
			return CasePacketSummary{}, err
		}
		for _, file := range caseFiles {
			if err := validateCasePacketName(file.Name, "case file name"); err != nil {
				return CasePacketSummary{}, err
			}
			archivePath := filepath.ToSlash(filepath.Join("case", file.Name))
			if archivePath == complaintArchivePath {
				continue
			}
			entries = append(entries, casePacketEntry{archivePath: archivePath, sourcePath: file.Path, role: "case_file"})
		}
	}
	slices.SortFunc(entries, func(a, b casePacketEntry) int {
		return strings.Compare(a.archivePath, b.archivePath)
	})
	control, err := casePacketControlBytes(caseFileMode, complaintArchivePath, explicitArchivePaths)
	if err != nil {
		return CasePacketSummary{}, err
	}
	manifest := CasePacketSummary{
		SchemaVersion: "aar.case-packet.v0",
		CaseFileMode:  caseFileMode,
		Complaint:     complaintArchivePath,
		CaseFiles:     explicitArchivePaths,
		Control: CasePacketControl{
			CaseArgs:       "control/case-args.txt",
			CaseArgsSHA384: sha384Bytes(control),
		},
	}
	for _, entry := range entries {
		info := complaintInfo
		if entry.sourcePath != complaintPath {
			var err error
			info, err = requirePacketRegularFile(entry.sourcePath, "case file")
			if err != nil {
				return CasePacketSummary{}, err
			}
		}
		sum, err := sha384File(entry.sourcePath)
		if err != nil {
			return CasePacketSummary{}, err
		}
		manifest.Files = append(manifest.Files, CasePacketFileMeta{
			ArchivePath: entry.archivePath,
			SourceName:  filepath.Base(entry.sourcePath),
			Role:        entry.role,
			SizeBytes:   info.Size(),
			SHA384:      sum,
		})
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return CasePacketSummary{}, fmt.Errorf("marshal case packet manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return CasePacketSummary{}, fmt.Errorf("write case packet manifest: %w", err)
	}
	if err := writeCasePacketArchive(packetPath, entries, control, manifestBytes); err != nil {
		return CasePacketSummary{}, err
	}
	packetInfo, err := os.Stat(packetPath)
	if err != nil {
		return CasePacketSummary{}, fmt.Errorf("stat case packet: %w", err)
	}
	packetSHA384, err := sha384File(packetPath)
	if err != nil {
		return CasePacketSummary{}, err
	}
	manifest.PacketSHA384 = packetSHA384
	manifest.PacketBytes = packetInfo.Size()
	return manifest, nil
}

func casePacketControlBytes(mode string, complaint string, files []string) ([]byte, error) {
	if err := validateCasePacketPath(complaint); err != nil {
		return nil, err
	}
	lines := []string{
		"case_file_mode=" + mode,
		"complaint=" + complaint,
	}
	for _, file := range files {
		if err := validateCasePacketPath(file); err != nil {
			return nil, err
		}
		lines = append(lines, "file="+file)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func writeCasePacketArchive(path string, entries []casePacketEntry, control []byte, manifest []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create case packet: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	gz.Name = ""
	gz.ModTime = time.Unix(0, 0).UTC()
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		if err := addCasePacketFile(tw, entry.archivePath, entry.sourcePath); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return err
		}
	}
	if err := addCasePacketBytes(tw, "control/case-args.txt", control); err != nil {
		_ = tw.Close()
		_ = gz.Close()
		return err
	}
	if err := addCasePacketBytes(tw, "control/case-packet.json", manifest); err != nil {
		_ = tw.Close()
		_ = gz.Close()
		return err
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return fmt.Errorf("close case packet tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close case packet gzip: %w", err)
	}
	closeErr := f.Close()
	closed = true
	if closeErr != nil {
		return fmt.Errorf("close case packet: %w", closeErr)
	}
	return nil
}

func addCasePacketFile(tw *tar.Writer, archivePath string, sourcePath string) error {
	if err := validateCasePacketPath(archivePath); err != nil {
		return err
	}
	info, err := requirePacketRegularFile(sourcePath, "case packet source")
	if err != nil {
		return err
	}
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open case packet source %s: %w", sourcePath, err)
	}
	defer f.Close()
	if err := writeCasePacketHeader(tw, archivePath, info.Size()); err != nil {
		return err
	}
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("write case packet member %s: %w", archivePath, err)
	}
	return nil
}

func addCasePacketBytes(tw *tar.Writer, archivePath string, data []byte) error {
	if err := validateCasePacketPath(archivePath); err != nil {
		return err
	}
	if err := writeCasePacketHeader(tw, archivePath, int64(len(data))); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write case packet member %s: %w", archivePath, err)
	}
	return nil
}

func writeCasePacketHeader(tw *tar.Writer, name string, size int64) error {
	header := &tar.Header{
		Name:     name,
		Mode:     0o644,
		Uid:      0,
		Gid:      0,
		Size:     size,
		ModTime:  time.Unix(0, 0).UTC(),
		Typeflag: tar.TypeReg,
		Format:   tar.FormatUSTAR,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write case packet header %s: %w", name, err)
	}
	return nil
}

func validateCasePacketName(value string, label string) error {
	if value == "" || value == "." || value == ".." || strings.Contains(value, "/") || strings.Contains(value, "\n") {
		return fmt.Errorf("invalid %s: %s", label, value)
	}
	return nil
}

func validateCasePacketPath(value string) error {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\n") {
		return fmt.Errorf("invalid packet path: %s", value)
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid packet path: %s", value)
		}
	}
	return nil
}

func requirePacketRegularFile(path string, label string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s %s: %w", label, path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %s is not a regular file", label, path)
	}
	return info, nil
}

func sha384File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha512.New384()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha384Bytes(data []byte) string {
	h := sha512.New384()
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
