package community

import (
	"context"
	"fmt"
	"strings"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// VersionChecker provides version comparison and update checking functionality
type VersionChecker struct {
	client *GitHubClient
}

// NewVersionChecker creates a new VersionChecker with the given client
func NewVersionChecker(client *GitHubClient) *VersionChecker {
	return &VersionChecker{
		client: client,
	}
}

// CheckPackUpdates compares installed packs with remote versions and returns available updates
func (v *VersionChecker) CheckPackUpdates(ctx context.Context, installed []domain.PackInfo) ([]domain.PackUpdate, error) {
	return v.client.CheckUpdates(ctx, installed)
}

// GetLatestVersion returns the latest version of a specific pack
func (v *VersionChecker) GetLatestVersion(ctx context.Context, packName string) (string, error) {
	return v.client.GetLatestVersion(ctx, packName)
}

// IsUpdateAvailable checks if an update is available for a specific pack
func (v *VersionChecker) IsUpdateAvailable(ctx context.Context, packName, currentVersion string) (bool, string, error) {
	latestVersion, err := v.client.GetLatestVersion(ctx, packName)
	if err != nil {
		return false, "", err
	}

	cmp, err := CompareSemVer(currentVersion, latestVersion)
	if err != nil {
		return false, "", fmt.Errorf("failed to compare versions: %w", err)
	}

	return cmp < 0, latestVersion, nil
}

// CompareSemVer compares two semantic version strings
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareSemVer(v1, v2 string) (int, error) {
	return compareSemVer(v1, v2)
}

// ParseSemVer parses a semantic version string into major, minor, patch components
func ParseSemVer(version string) (major, minor, patch int, err error) {
	parts := parseVersion(version)
	if len(parts) >= 1 {
		major = parts[0]
	}
	if len(parts) >= 2 {
		minor = parts[1]
	}
	if len(parts) >= 3 {
		patch = parts[2]
	}
	return major, minor, patch, nil
}

// SatisfiesConstraint checks if a version satisfies a version constraint
// Supports constraints like: ">=1.0.0", "<=2.0.0", "=1.0.0", ">1.0.0", "<2.0.0"
func SatisfiesConstraint(version, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true, nil
	}

	// Parse operator and version from constraint
	var op string
	var constraintVersion string

	if strings.HasPrefix(constraint, ">=") {
		op = ">="
		constraintVersion = strings.TrimPrefix(constraint, ">=")
	} else if strings.HasPrefix(constraint, "<=") {
		op = "<="
		constraintVersion = strings.TrimPrefix(constraint, "<=")
	} else if strings.HasPrefix(constraint, ">") {
		op = ">"
		constraintVersion = strings.TrimPrefix(constraint, ">")
	} else if strings.HasPrefix(constraint, "<") {
		op = "<"
		constraintVersion = strings.TrimPrefix(constraint, "<")
	} else if strings.HasPrefix(constraint, "=") {
		op = "="
		constraintVersion = strings.TrimPrefix(constraint, "=")
	} else {
		// No operator, assume exact match
		op = "="
		constraintVersion = constraint
	}

	constraintVersion = strings.TrimSpace(constraintVersion)

	cmp, err := CompareSemVer(version, constraintVersion)
	if err != nil {
		return false, err
	}

	switch op {
	case ">=":
		return cmp >= 0, nil
	case "<=":
		return cmp <= 0, nil
	case ">":
		return cmp > 0, nil
	case "<":
		return cmp < 0, nil
	case "=":
		return cmp == 0, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// FormatVersion formats version components into a semantic version string
func FormatVersion(major, minor, patch int) string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

// IncrementPatch increments the patch version
func IncrementPatch(version string) (string, error) {
	major, minor, patch, err := ParseSemVer(version)
	if err != nil {
		return "", err
	}
	return FormatVersion(major, minor, patch+1), nil
}

// IncrementMinor increments the minor version and resets patch
func IncrementMinor(version string) (string, error) {
	major, minor, _, err := ParseSemVer(version)
	if err != nil {
		return "", err
	}
	return FormatVersion(major, minor+1, 0), nil
}

// IncrementMajor increments the major version and resets minor and patch
func IncrementMajor(version string) (string, error) {
	major, _, _, err := ParseSemVer(version)
	if err != nil {
		return "", err
	}
	return FormatVersion(major+1, 0, 0), nil
}
