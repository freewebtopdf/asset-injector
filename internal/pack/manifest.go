// Package pack provides functionality for managing rule packs in the Asset Injector.
// It handles manifest parsing, validation, namespace prefixing, and pack lifecycle management.
package pack

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"gopkg.in/yaml.v3"
)

// ManifestFileName is the standard name for pack manifest files
const ManifestFileName = "manifest.yaml"

// ManifestParser handles parsing and validation of pack manifest files
type ManifestParser struct{}

// NewManifestParser creates a new ManifestParser instance
func NewManifestParser() *ManifestParser {
	return &ManifestParser{}
}

// ParseFile reads and parses a manifest file from the given path
func (p *ManifestParser) ParseFile(path string) (*domain.PackManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	return p.Parse(data)
}

// Parse parses manifest content from bytes
func (p *ManifestParser) Parse(data []byte) (*domain.PackManifest, error) {
	var manifest domain.PackManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	return &manifest, nil
}

// ManifestValidator validates pack manifests against required schema
type ManifestValidator struct{}

// NewManifestValidator creates a new ManifestValidator instance
func NewManifestValidator() *ManifestValidator {
	return &ManifestValidator{}
}

// ValidationError represents a manifest validation error with details
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("manifest validation failed: %s", strings.Join(msgs, "; "))
}

// Validate validates a manifest and returns any validation errors
func (v *ManifestValidator) Validate(manifest *domain.PackManifest) error {
	var errors ValidationErrors

	// Check required fields
	if manifest.Name == "" {
		errors = append(errors, ValidationError{Field: "name", Message: "required field is missing"})
	} else if !isValidPackName(manifest.Name) {
		errors = append(errors, ValidationError{Field: "name", Message: "must be lowercase alphanumeric with hyphens only"})
	}

	if manifest.Version == "" {
		errors = append(errors, ValidationError{Field: "version", Message: "required field is missing"})
	} else if !IsValidSemVer(manifest.Version) {
		errors = append(errors, ValidationError{Field: "version", Message: "must be a valid semantic version (e.g., 1.0.0)"})
	}

	if manifest.Description == "" {
		errors = append(errors, ValidationError{Field: "description", Message: "required field is missing"})
	}

	if manifest.Author == "" {
		errors = append(errors, ValidationError{Field: "author", Message: "required field is missing"})
	}

	// Validate dependencies if present
	for i, dep := range manifest.Dependencies {
		if dep.Name == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("dependencies[%d].name", i),
				Message: "required field is missing",
			})
		}
		if dep.Version == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("dependencies[%d].version", i),
				Message: "required field is missing",
			})
		} else if !isValidVersionConstraint(dep.Version) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("dependencies[%d].version", i),
				Message: "must be a valid version constraint (e.g., >=1.0.0)",
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// isValidPackName checks if a pack name follows naming conventions
func isValidPackName(name string) bool {
	// Pack names should be lowercase alphanumeric with hyphens
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`, name)
	return matched
}

// isValidVersionConstraint checks if a version constraint is valid
func isValidVersionConstraint(constraint string) bool {
	// Support formats: "1.0.0", ">=1.0.0", "<=1.0.0", ">1.0.0", "<1.0.0", "^1.0.0", "~1.0.0"
	constraint = strings.TrimSpace(constraint)

	// Remove operator prefix if present
	operators := []string{">=", "<=", ">", "<", "^", "~", "="}
	version := constraint
	for _, op := range operators {
		if strings.HasPrefix(constraint, op) {
			version = strings.TrimPrefix(constraint, op)
			break
		}
	}

	return IsValidSemVer(version)
}

// SemVer represents a parsed semantic version
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// String returns the string representation of the semantic version
func (v SemVer) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// semVerRegex matches semantic version strings
var semVerRegex = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// IsValidSemVer checks if a string is a valid semantic version
func IsValidSemVer(version string) bool {
	return semVerRegex.MatchString(version)
}

// ParseSemVer parses a semantic version string into a SemVer struct
func ParseSemVer(version string) (*SemVer, error) {
	matches := semVerRegex.FindStringSubmatch(version)
	if matches == nil {
		return nil, fmt.Errorf("invalid semantic version: %s", version)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// CompareSemVer compares two semantic versions
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareSemVer(v1, v2 string) (int, error) {
	sv1, err := ParseSemVer(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version v1: %w", err)
	}

	sv2, err := ParseSemVer(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version v2: %w", err)
	}

	return sv1.Compare(sv2), nil
}

// Compare compares this version with another
// Returns -1 if this < other, 0 if this == other, 1 if this > other
func (v *SemVer) Compare(other *SemVer) int {
	// Compare major version
	if v.Major < other.Major {
		return -1
	}
	if v.Major > other.Major {
		return 1
	}

	// Compare minor version
	if v.Minor < other.Minor {
		return -1
	}
	if v.Minor > other.Minor {
		return 1
	}

	// Compare patch version
	if v.Patch < other.Patch {
		return -1
	}
	if v.Patch > other.Patch {
		return 1
	}

	// Compare prerelease (empty prerelease > any prerelease)
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease < other.Prerelease {
		return -1
	}
	if v.Prerelease > other.Prerelease {
		return 1
	}

	return 0
}

// IsNewerThan returns true if this version is newer than the other
func (v *SemVer) IsNewerThan(other *SemVer) bool {
	return v.Compare(other) > 0
}

// IsOlderThan returns true if this version is older than the other
func (v *SemVer) IsOlderThan(other *SemVer) bool {
	return v.Compare(other) < 0
}

// SatisfiesConstraint checks if a version satisfies a version constraint
func SatisfiesConstraint(version, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)

	// Determine operator and target version
	var operator string
	var targetVersion string

	operators := []string{">=", "<=", ">", "<", "^", "~", "="}
	for _, op := range operators {
		if strings.HasPrefix(constraint, op) {
			operator = op
			targetVersion = strings.TrimPrefix(constraint, op)
			break
		}
	}

	// If no operator, treat as exact match
	if operator == "" {
		operator = "="
		targetVersion = constraint
	}

	cmp, err := CompareSemVer(version, targetVersion)
	if err != nil {
		return false, err
	}

	switch operator {
	case "=":
		return cmp == 0, nil
	case ">":
		return cmp > 0, nil
	case "<":
		return cmp < 0, nil
	case ">=":
		return cmp >= 0, nil
	case "<=":
		return cmp <= 0, nil
	case "^":
		// Caret: compatible with version (same major, >= minor.patch)
		v, _ := ParseSemVer(version)
		t, _ := ParseSemVer(targetVersion)
		return v.Major == t.Major && cmp >= 0, nil
	case "~":
		// Tilde: approximately equivalent (same major.minor, >= patch)
		v, _ := ParseSemVer(version)
		t, _ := ParseSemVer(targetVersion)
		return v.Major == t.Major && v.Minor == t.Minor && cmp >= 0, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

// ManifestToPackInfo converts a manifest to PackInfo
func ManifestToPackInfo(manifest *domain.PackManifest, ruleCount int) domain.PackInfo {
	return domain.PackInfo{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		Author:      manifest.Author,
		License:     manifest.License,
		Homepage:    manifest.Homepage,
		Tags:        manifest.Tags,
		RuleCount:   ruleCount,
	}
}
