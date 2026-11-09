package services

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// CertificateService handles SSL certificate operations
type CertificateService struct {
	dataDir string
}

// NewCertificateService creates a new certificate service
func NewCertificateService(dataDir string) *CertificateService {
	return &CertificateService{
		dataDir: dataDir,
	}
}

// GetAllCertificates returns all certificates
func (c *CertificateService) GetAllCertificates() ([]*models.Certificate, error) {
	certsDir := filepath.Join(c.dataDir, "caddy", "certificates")

	if _, err := os.Stat(certsDir); os.IsNotExist(err) {
		return []*models.Certificate{}, nil
	}

	var certs []*models.Certificate

	// Walk through certificate directories
	err := filepath.Walk(certsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".crt") {
			return nil
		}

		cert, err := c.parseCertificate(path)
		if err != nil {
			// Log but continue
			log.Printf("Warning: Could not parse certificate %s: %v", path, err)
			return nil
		}

		certs = append(certs, cert)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk certificates directory: %w", err)
	}

	// Sort by domain
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].Domain < certs[j].Domain
	})

	return certs, nil
}

// GetCertificate returns a single certificate by domain
func (c *CertificateService) GetCertificate(domain string) (*models.Certificate, error) {
	certs, err := c.GetAllCertificates()
	if err != nil {
		return nil, err
	}

	for _, cert := range certs {
		if cert.Domain == domain {
			return cert, nil
		}
	}

	return nil, fmt.Errorf("certificate not found for domain: %s", domain)
}

// DeleteCertificate removes a certificate from Caddy's storage by moving its
// files (.crt, .key, .json) into the certificate backup directory, so it can
// be restored if Caddy fails to obtain a new one. Caddy keeps serving the
// certificate from memory until it is restarted.
func (c *CertificateService) DeleteCertificate(domain, reason string) (*CertBackup, error) {
	cert, err := c.GetCertificate(domain)
	if err != nil {
		return nil, err
	}
	return c.backupCertificate(cert, reason)
}

// parseCertificate parses a certificate file
func (c *CertificateService) parseCertificate(path string) (*models.Certificate, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(content)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Get domain from path or certificate
	domain := c.extractDomain(path, cert)

	result := &models.Certificate{
		Domain:       domain,
		Issuer:       cert.Issuer.CommonName,
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		SerialNumber: cert.SerialNumber.String(),
		FilePath:     path,
	}

	result.UpdateStatus()

	return result, nil
}

// extractDomain extracts domain from path or certificate
func (c *CertificateService) extractDomain(path string, cert *x509.Certificate) string {
	// Try to get from path
	// Typical path: .../certificates/acme-v02.api.letsencrypt.org-directory/example.com/example.com.crt
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) >= 2 {
		dir := parts[len(parts)-2]
		if dir != "certificates" && !strings.Contains(dir, "acme") {
			return dir
		}
	}

	// Fall back to certificate CN or first DNS name
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}

	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}

	return "unknown"
}

// GetStats returns certificate statistics
func (c *CertificateService) GetStats() map[string]int {
	certs, _ := c.GetAllCertificates()

	stats := map[string]int{
		"total":    len(certs),
		"valid":    0,
		"expiring": 0,
		"critical": 0,
		"expired":  0,
	}

	for _, cert := range certs {
		switch cert.Status {
		case models.CertStatusValid:
			stats["valid"]++
		case models.CertStatusExpiring:
			stats["expiring"]++
		case models.CertStatusCritical:
			stats["critical"]++
		case models.CertStatusExpired:
			stats["expired"]++
		}
	}

	return stats
}

// GetExpiringCertificates returns certificates expiring within given days
func (c *CertificateService) GetExpiringCertificates(days int) ([]*models.Certificate, error) {
	certs, err := c.GetAllCertificates()
	if err != nil {
		return nil, err
	}

	var expiring []*models.Certificate
	for _, cert := range certs {
		if cert.DaysLeft <= days {
			expiring = append(expiring, cert)
		}
	}

	// Sort by days left (most urgent first)
	sort.Slice(expiring, func(i, j int) bool {
		return expiring[i].DaysLeft < expiring[j].DaysLeft
	})

	return expiring, nil
}
