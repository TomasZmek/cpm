package models

import (
	"strings"
	"time"
)

// CertificateStatus represents the status of a certificate
type CertificateStatus string

const (
	CertStatusValid    CertificateStatus = "valid"
	CertStatusExpiring CertificateStatus = "expiring"
	CertStatusCritical CertificateStatus = "critical"
	CertStatusExpired  CertificateStatus = "expired"
	CertStatusUnknown  CertificateStatus = "unknown"
)

// Certificate represents an SSL certificate
type Certificate struct {
	Domain       string            `json:"domain"`
	Issuer       string            `json:"issuer"`
	NotBefore    time.Time         `json:"not_before"`
	NotAfter     time.Time         `json:"not_after"`
	SerialNumber string            `json:"serial_number"`
	FilePath     string            `json:"file_path"`
	Status       CertificateStatus `json:"status"`
	DaysLeft     int               `json:"days_left"`
}

// UpdateStatus updates the certificate status based on expiration
func (c *Certificate) UpdateStatus() {
	now := time.Now()
	c.DaysLeft = int(c.NotAfter.Sub(now).Hours() / 24)

	switch {
	case c.NotAfter.Before(now):
		c.Status = CertStatusExpired
	case c.DaysLeft <= 7:
		c.Status = CertStatusCritical
	case c.DaysLeft <= 30:
		c.Status = CertStatusExpiring
	default:
		c.Status = CertStatusValid
	}
}

// StatusIcon returns icon for the certificate status
func (c *Certificate) StatusIcon() string {
	switch c.Status {
	case CertStatusValid:
		return "✅"
	case CertStatusExpiring:
		return "⚠️"
	case CertStatusCritical:
		return "🔴"
	case CertStatusExpired:
		return "❌"
	default:
		return "❓"
	}
}

// StatusClass returns CSS class for the certificate status
func (c *Certificate) StatusClass() string {
	switch c.Status {
	case CertStatusValid:
		return "text-green-600"
	case CertStatusExpiring:
		return "text-yellow-600"
	case CertStatusCritical:
		return "text-orange-600"
	case CertStatusExpired:
		return "text-red-600"
	default:
		return "text-gray-600"
	}
}

// IsWildcard reports whether this is a wildcard certificate. Caddy stores
// them in directories named "wildcard_.example.com".
func (c *Certificate) IsWildcard() bool {
	return strings.HasPrefix(c.Domain, "*.") || strings.HasPrefix(c.Domain, "wildcard_")
}
