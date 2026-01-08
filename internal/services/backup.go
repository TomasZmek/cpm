package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/models"
)

// BackupInfo contains information about a backup
type BackupInfo struct {
	FilesCount int
	Size       int64
	SizeHuman  string
	CreatedAt  time.Time
}

// RestoreResult contains the result of a restore operation
type RestoreResult struct {
	Success bool
	Message string
	Errors  []string
}

// BackupService handles backup and restore operations
type BackupService struct {
	config *config.Config
}

// NewBackupService creates a new backup service
func NewBackupService(cfg *config.Config) *BackupService {
	return &BackupService{
		config: cfg,
	}
}

// CreateBackup creates a ZIP backup of all configuration
func (b *BackupService) CreateBackup() ([]byte, string, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Files to backup
	filesToBackup := []string{
		"Caddyfile",
		"snippets.caddy",
		".snippets_config.json",
	}

	// Add individual files
	for _, filename := range filesToBackup {
		path := filepath.Join(b.config.ConfigDir, filename)
		if err := b.addFileToZip(zipWriter, path, filename); err != nil {
			// Log but continue - file might not exist
			fmt.Printf("Skipping %s: %v\n", filename, err)
		}
	}

	// Add sites directory
	sitesDir := b.config.SitesDir
	if _, err := os.Stat(sitesDir); err == nil {
		entries, _ := os.ReadDir(sitesDir)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".caddy") {
				path := filepath.Join(sitesDir, entry.Name())
				zipPath := "sites/" + entry.Name()
				b.addFileToZip(zipWriter, path, zipPath)
			}
		}
	}

	// Add pages directory
	pagesDir := filepath.Join(b.config.ConfigDir, "pages")
	if _, err := os.Stat(pagesDir); err == nil {
		entries, _ := os.ReadDir(pagesDir)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
				path := filepath.Join(pagesDir, entry.Name())
				zipPath := "pages/" + entry.Name()
				b.addFileToZip(zipWriter, path, zipPath)
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close zip: %w", err)
	}

	filename := fmt.Sprintf("cpm_backup_%s.zip", time.Now().Format("2006-01-02_15-04-05"))
	return buf.Bytes(), filename, nil
}

// GetBackupInfo returns information about a backup
func (b *BackupService) GetBackupInfo(data []byte) (*BackupInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip file: %w", err)
	}

	info := &BackupInfo{
		FilesCount: len(reader.File),
		Size:       int64(len(data)),
		CreatedAt:  time.Now(),
	}

	// Find oldest file time as creation time
	for _, f := range reader.File {
		if f.Modified.Before(info.CreatedAt) {
			info.CreatedAt = f.Modified
		}
	}

	// Human readable size
	info.SizeHuman = formatBytes(info.Size)

	return info, nil
}

// RestoreBackup restores configuration from a ZIP backup
func (b *BackupService) RestoreBackup(data []byte) *RestoreResult {
	result := &RestoreResult{
		Success: true,
		Errors:  []string{},
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		result.Success = false
		result.Message = "Invalid zip file"
		return result
	}

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Determine target path
		var targetPath string
		switch {
		case strings.HasPrefix(f.Name, "sites/"):
			targetPath = filepath.Join(b.config.SitesDir, strings.TrimPrefix(f.Name, "sites/"))
		case strings.HasPrefix(f.Name, "pages/"):
			targetPath = filepath.Join(b.config.ConfigDir, f.Name)
		default:
			targetPath = filepath.Join(b.config.ConfigDir, f.Name)
		}

		// Create directory if needed
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to create directory for %s: %v", f.Name, err))
			continue
		}

		// Extract file
		if err := b.extractFile(f, targetPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to extract %s: %v", f.Name, err))
		}
	}

	if len(result.Errors) > 0 {
		result.Message = fmt.Sprintf("Restored with %d errors", len(result.Errors))
	} else {
		result.Message = "Backup restored successfully"
	}

	return result
}

// ExportRules exports all rules as JSON
func (b *BackupService) ExportRules(sites []*models.Site) ([]byte, error) {
	type exportSite struct {
		Filename           string   `json:"filename"`
		Domains            []string `json:"domains"`
		TargetIP           string   `json:"target_ip"`
		TargetPort         string   `json:"target_port"`
		IsHTTPSBackend     bool     `json:"is_https_backend"`
		IsInternal         bool     `json:"is_internal"`
		Snippets           []string `json:"snippets"`
		Tags               []string `json:"tags"`
		AdditionalBackends []string `json:"additional_backends"`
		LBPolicy           string   `json:"lb_policy"`
		EnableWebSocket    bool     `json:"enable_websocket"`
		HealthCheckPath    string   `json:"health_check_path"`
		TimeoutSeconds     int      `json:"timeout_seconds"`
		ExtraConfig        string   `json:"extra_config"`
		RawContent         string   `json:"raw_content"`
	}

	var export []exportSite
	for _, site := range sites {
		export = append(export, exportSite{
			Filename:           site.Filename,
			Domains:            site.Domains,
			TargetIP:           site.TargetIP,
			TargetPort:         site.TargetPort,
			IsHTTPSBackend:     site.IsHTTPSBackend,
			IsInternal:         site.IsInternal,
			Snippets:           site.Snippets,
			Tags:               site.Tags,
			AdditionalBackends: site.AdditionalBackends,
			LBPolicy:           site.LBPolicy,
			EnableWebSocket:    site.EnableWebSocket,
			HealthCheckPath:    site.HealthCheckPath,
			TimeoutSeconds:     site.TimeoutSeconds,
			ExtraConfig:        site.ExtraConfig,
			RawContent:         site.RawContent,
		})
	}

	return json.MarshalIndent(export, "", "  ")
}

// ImportRules imports rules from JSON
func (b *BackupService) ImportRules(data []byte, caddyService *CaddyService, skipExisting bool) (int, int, error) {
	var rules []map[string]interface{}
	if err := json.Unmarshal(data, &rules); err != nil {
		return 0, 0, fmt.Errorf("invalid JSON: %w", err)
	}

	existingSites, _ := caddyService.GetAllSites()
	existingNames := make(map[string]bool)
	for _, s := range existingSites {
		existingNames[s.Filename] = true
	}

	imported := 0
	skipped := 0

	for _, rule := range rules {
		filename, _ := rule["filename"].(string)

		if skipExisting && existingNames[filename] {
			skipped++
			continue
		}

		// Create site from rule
		site := &models.Site{
			Filename: filename,
		}

		if domains, ok := rule["domains"].([]interface{}); ok {
			for _, d := range domains {
				if ds, ok := d.(string); ok {
					site.Domains = append(site.Domains, ds)
				}
			}
		}

		if v, ok := rule["target_ip"].(string); ok {
			site.TargetIP = v
		}
		if v, ok := rule["target_port"].(string); ok {
			site.TargetPort = v
		}
		if v, ok := rule["is_https_backend"].(bool); ok {
			site.IsHTTPSBackend = v
		}
		if v, ok := rule["is_internal"].(bool); ok {
			site.IsInternal = v
		}

		if snippets, ok := rule["snippets"].([]interface{}); ok {
			for _, s := range snippets {
				if ss, ok := s.(string); ok {
					site.Snippets = append(site.Snippets, ss)
				}
			}
		}

		if tags, ok := rule["tags"].([]interface{}); ok {
			for _, t := range tags {
				if ts, ok := t.(string); ok {
					site.Tags = append(site.Tags, ts)
				}
			}
		}

		// Use raw content if available
		if raw, ok := rule["raw_content"].(string); ok && raw != "" {
			site.RawContent = raw
			caddyService.UpdateSiteRaw(filename, raw)
		} else {
			caddyService.CreateSite(site)
		}

		imported++
	}

	return imported, skipped, nil
}

// addFileToZip adds a file to the zip archive
func (b *BackupService) addFileToZip(zw *zip.Writer, sourcePath, zipPath string) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write(content)
	return err
}

// extractFile extracts a file from the zip
func (b *BackupService) extractFile(f *zip.File, targetPath string) error {
	reader, err := f.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, content, 0644)
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
