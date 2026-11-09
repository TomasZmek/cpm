package services

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTestCert stores a self-signed certificate the way Caddy does:
// certificates/<issuer>/<name>/<name>.{crt,key,json}
func writeTestCert(t *testing.T, dataDir, name, cn string, serial int64) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     []string{cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(dataDir, "caddy", "certificates", "acme-v02.api.letsencrypt.org-directory", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	crt := filepath.Join(dir, name+".crt")
	os.WriteFile(crt, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(filepath.Join(dir, name+".key"), []byte("key"), 0o600)
	os.WriteFile(filepath.Join(dir, name+".json"), []byte("{}"), 0o600)
	return dir
}

func TestCertificateBackupRestoreRoundTrip(t *testing.T) {
	data := t.TempDir()
	svc := NewCertificateService(data)
	dir := writeTestCert(t, data, "wildcard_.example.com", "*.example.com", 1)

	b, err := svc.DeleteCertificate("wildcard_.example.com", CertBackupReasonRenew)
	if err != nil {
		t.Fatalf("DeleteCertificate: %v", err)
	}
	for _, ext := range certFileExts {
		if _, err := os.Stat(filepath.Join(dir, "wildcard_.example.com"+ext)); !os.IsNotExist(err) {
			t.Fatalf("%s still in Caddy storage", ext)
		}
	}
	if certs, _ := svc.GetAllCertificates(); len(certs) != 0 {
		t.Fatalf("backup must not be listed as an active certificate, got %d", len(certs))
	}

	backups, err := svc.ListBackups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("ListBackups = %v, %v", backups, err)
	}
	if backups[0].ID != b.ID || backups[0].Reason != CertBackupReasonRenew || backups[0].IsExpired() {
		t.Fatalf("unexpected backup %+v", backups[0])
	}

	// Caddy obtained a new certificate meanwhile.
	writeTestCert(t, data, "wildcard_.example.com", "*.example.com", 2)

	if _, err := svc.RestoreBackup(b.ID); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	cert, err := svc.GetCertificate("wildcard_.example.com")
	if err != nil || cert.SerialNumber != "1" {
		t.Fatalf("restored certificate = %+v, %v", cert, err)
	}
	for _, ext := range certFileExts {
		if _, err := os.Stat(filepath.Join(dir, "wildcard_.example.com"+ext)); err != nil {
			t.Fatalf("%s not restored: %v", ext, err)
		}
	}
	// The replaced (newer) certificate is kept as a backup, the restored one is gone.
	backups, _ = svc.ListBackups()
	if len(backups) != 1 || backups[0].Reason != CertBackupReasonRestore || backups[0].SerialNumber != "2" {
		t.Fatalf("after restore backups = %+v", backups)
	}

	if err := svc.DeleteBackup(backups[0].ID); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	if backups, _ = svc.ListBackups(); len(backups) != 0 {
		t.Fatalf("backup not deleted")
	}
}

func TestCertificateBackupRejectsBadInput(t *testing.T) {
	data := t.TempDir()
	svc := NewCertificateService(data)
	writeTestCert(t, data, "example.com", "example.com", 1)
	b, err := svc.DeleteCertificate("example.com", CertBackupReasonDelete)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"", "..", "../caddy", "20261109T120000-a/../../x", "x"} {
		if _, err := svc.RestoreBackup(id); err == nil {
			t.Errorf("RestoreBackup(%q) accepted", id)
		}
		if err := svc.DeleteBackup(id); err == nil {
			t.Errorf("DeleteBackup(%q) accepted", id)
		}
	}

	// A tampered backup.json must not write outside Caddy's storage.
	meta := filepath.Join(data, certBackupDir, b.ID, "backup.json")
	os.WriteFile(meta, []byte(`{"domain":"x","rel_dir":"../../etc","base":"example.com"}`), 0o600)
	if _, err := svc.RestoreBackup(b.ID); err == nil {
		t.Fatal("restore with rel_dir outside storage accepted")
	}
}
