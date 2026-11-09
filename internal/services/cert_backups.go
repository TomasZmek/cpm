package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
)

// Reasons recorded with a certificate backup.
const (
	CertBackupReasonRenew   = "renew"
	CertBackupReasonDelete  = "delete"
	CertBackupReasonRestore = "restore"
)

// certBackupDir is the directory (inside the Caddy data dir, outside Caddy's
// own storage) that holds certificates removed by CPM.
const certBackupDir = "cpm-cert-backups"

var (
	certBackupIDPattern   = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}-[a-z0-9._-]+(-[0-9]+)?$`)
	certBackupBasePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// certFileExts are the files Caddy stores for one certificate.
var certFileExts = []string{".crt", ".key", ".json"}

// CertBackup is a certificate that CPM moved out of Caddy's storage.
type CertBackup struct {
	ID           string                   `json:"-"`
	Domain       string                   `json:"domain"`
	Issuer       string                   `json:"issuer"`
	NotBefore    time.Time                `json:"not_before"`
	NotAfter     time.Time                `json:"not_after"`
	SerialNumber string                   `json:"serial_number"`
	RelDir       string                   `json:"rel_dir"` // directory relative to caddy/certificates
	Base         string                   `json:"base"`    // file name without extension
	BackedUpAt   time.Time                `json:"backed_up_at"`
	Reason       string                   `json:"reason"`
	Status       models.CertificateStatus `json:"-"`
	DaysLeft     int                      `json:"-"`
}

// IsExpired reports whether the backed-up certificate is no longer valid.
func (b *CertBackup) IsExpired() bool { return b.Status == models.CertStatusExpired }

func (c *CertificateService) certsDir() string {
	return filepath.Join(c.dataDir, "caddy", "certificates")
}

func (c *CertificateService) backupRoot() string {
	return filepath.Join(c.dataDir, certBackupDir)
}

// backupCertificate moves the certificate's files into a new backup.
func (c *CertificateService) backupCertificate(cert *models.Certificate, reason string) (*CertBackup, error) {
	relDir, err := filepath.Rel(c.certsDir(), filepath.Dir(cert.FilePath))
	if err != nil || !filepath.IsLocal(relDir) {
		return nil, fmt.Errorf("certificate outside of Caddy storage: %s", cert.FilePath)
	}
	base := strings.TrimSuffix(filepath.Base(cert.FilePath), ".crt")
	if !certBackupBasePattern.MatchString(base) {
		return nil, fmt.Errorf("unexpected certificate file name: %s", base)
	}

	b := &CertBackup{
		Domain:       cert.Domain,
		Issuer:       cert.Issuer,
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		SerialNumber: cert.SerialNumber,
		RelDir:       filepath.ToSlash(relDir),
		Base:         base,
		BackedUpAt:   time.Now().UTC(),
		Reason:       reason,
	}

	if err := os.MkdirAll(c.backupRoot(), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}
	id := b.BackedUpAt.Format("20060102T150405") + "-" + strings.ToLower(base)
	dir := filepath.Join(c.backupRoot(), id)
	for n := 2; ; n++ {
		if err := os.Mkdir(dir, 0o700); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
		id = fmt.Sprintf("%s-%s-%d", b.BackedUpAt.Format("20060102T150405"), strings.ToLower(base), n)
		dir = filepath.Join(c.backupRoot(), id)
	}
	b.ID = id

	meta, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "backup.json"), meta, 0o600); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to write backup: %w", err)
	}

	srcDir := filepath.Dir(cert.FilePath)
	for _, ext := range certFileExts {
		src := filepath.Join(srcDir, base+ext)
		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := moveFile(src, filepath.Join(dir, base+ext)); err != nil {
			// Put back what was already moved; the certificate stays in place.
			c.moveBack(dir, srcDir, base)
			os.RemoveAll(dir)
			return nil, fmt.Errorf("failed to back up certificate: %w", err)
		}
	}
	b.updateStatus()
	return b, nil
}

// moveBack moves any certificate files from a backup directory to dstDir.
func (c *CertificateService) moveBack(backupDir, dstDir, base string) error {
	var firstErr error
	for _, ext := range certFileExts {
		src := filepath.Join(backupDir, base+ext)
		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := moveFile(src, filepath.Join(dstDir, base+ext)); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ListBackups returns certificate backups, newest first.
func (c *CertificateService) ListBackups() ([]*CertBackup, error) {
	entries, err := os.ReadDir(c.backupRoot())
	if errors.Is(err, os.ErrNotExist) {
		return []*CertBackup{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := []*CertBackup{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := c.GetBackup(e.Name())
		if err != nil {
			continue
		}
		backups = append(backups, b)
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].BackedUpAt.After(backups[j].BackedUpAt)
	})
	return backups, nil
}

// GetBackup loads and validates one backup.
func (c *CertificateService) GetBackup(id string) (*CertBackup, error) {
	if !certBackupIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid backup id")
	}
	data, err := os.ReadFile(filepath.Join(c.backupRoot(), id, "backup.json"))
	if err != nil {
		return nil, fmt.Errorf("backup not found")
	}
	var b CertBackup
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("invalid backup")
	}
	if !filepath.IsLocal(filepath.FromSlash(b.RelDir)) || !certBackupBasePattern.MatchString(b.Base) {
		return nil, fmt.Errorf("invalid backup")
	}
	if _, err := os.Stat(filepath.Join(c.backupRoot(), id, b.Base+".crt")); err != nil {
		return nil, fmt.Errorf("backup is incomplete")
	}
	b.ID = id
	b.updateStatus()
	return &b, nil
}

// RestoreBackup moves a backed-up certificate back into Caddy's storage. A
// certificate currently stored for the same name is backed up first, so
// nothing is lost. Caddy must be restarted to load the restored certificate.
func (c *CertificateService) RestoreBackup(id string) (*CertBackup, error) {
	b, err := c.GetBackup(id)
	if err != nil {
		return nil, err
	}
	dstDir := filepath.Join(c.certsDir(), filepath.FromSlash(b.RelDir))
	current := filepath.Join(dstDir, b.Base+".crt")
	if _, err := os.Stat(current); err == nil {
		cert, err := c.parseCertificate(current)
		if err != nil {
			return nil, fmt.Errorf("cannot back up the current certificate: %w", err)
		}
		if _, err := c.backupCertificate(cert, CertBackupReasonRestore); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to restore certificate: %w", err)
	}
	backupDir := filepath.Join(c.backupRoot(), id)
	if err := c.moveBack(backupDir, dstDir, b.Base); err != nil {
		return nil, fmt.Errorf("failed to restore certificate: %w", err)
	}
	os.RemoveAll(backupDir)
	return b, nil
}

// DeleteBackup permanently removes a certificate backup.
func (c *CertificateService) DeleteBackup(id string) error {
	if _, err := c.GetBackup(id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(c.backupRoot(), id))
}

func (b *CertBackup) updateStatus() {
	cert := models.Certificate{NotAfter: b.NotAfter}
	cert.UpdateStatus()
	b.Status = cert.Status
	b.DaysLeft = cert.DaysLeft
}

// moveFile renames src to dst, falling back to copy+remove across devices.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return os.Remove(src)
}
