package pack

import (
	"context"
	"fmt"
	"slices"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// DependencyChecker validates pack dependencies
type DependencyChecker struct {
	manager *PackManager
}

// NewDependencyChecker creates a new DependencyChecker
func NewDependencyChecker(manager *PackManager) *DependencyChecker {
	return &DependencyChecker{manager: manager}
}

// DependencyStatus represents the status of a dependency check
type DependencyStatus struct {
	Name             string `json:"name"`
	RequiredVersion  string `json:"required_version"`
	InstalledVersion string `json:"installed_version,omitempty"`
	Satisfied        bool   `json:"satisfied"`
	Missing          bool   `json:"missing"`
	Message          string `json:"message,omitempty"`
}

// DependencyCheckResult contains the results of checking all dependencies
type DependencyCheckResult struct {
	AllSatisfied bool               `json:"all_satisfied"`
	Dependencies []DependencyStatus `json:"dependencies"`
	Warnings     []string           `json:"warnings,omitempty"`
}

// CheckDependencies validates that all dependencies for a manifest are satisfied
// Returns warnings for missing dependencies but does not fail
func (c *DependencyChecker) CheckDependencies(ctx context.Context, manifest *domain.PackManifest) (*DependencyCheckResult, error) {
	result := &DependencyCheckResult{
		AllSatisfied: true,
		Dependencies: make([]DependencyStatus, 0, len(manifest.Dependencies)),
	}

	if len(manifest.Dependencies) == 0 {
		return result, nil
	}

	for _, dep := range manifest.Dependencies {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		status := c.checkSingleDependency(dep)
		result.Dependencies = append(result.Dependencies, status)

		if !status.Satisfied {
			result.AllSatisfied = false
			if status.Missing {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Missing dependency: %s (requires %s)", dep.Name, dep.Version))
			} else {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Version mismatch for %s: installed %s, requires %s",
						dep.Name, status.InstalledVersion, dep.Version))
			}
		}
	}

	return result, nil
}

// checkSingleDependency checks if a single dependency is satisfied
func (c *DependencyChecker) checkSingleDependency(dep domain.PackDependency) DependencyStatus {
	status := DependencyStatus{
		Name:            dep.Name,
		RequiredVersion: dep.Version,
	}

	// Check if the dependency is installed
	if !c.manager.IsInstalled(dep.Name) {
		status.Missing = true
		status.Satisfied = false
		status.Message = "Pack is not installed"
		return status
	}

	// Get installed version
	installedVersion, err := c.manager.GetInstalledVersion(dep.Name)
	if err != nil {
		status.Missing = true
		status.Satisfied = false
		status.Message = fmt.Sprintf("Failed to get installed version: %v", err)
		return status
	}

	status.InstalledVersion = installedVersion

	// Check if version satisfies constraint
	satisfied, err := SatisfiesConstraint(installedVersion, dep.Version)
	if err != nil {
		status.Satisfied = false
		status.Message = fmt.Sprintf("Failed to check version constraint: %v", err)
		return status
	}

	status.Satisfied = satisfied
	if satisfied {
		status.Message = "Dependency satisfied"
	} else {
		status.Message = fmt.Sprintf("Installed version %s does not satisfy %s", installedVersion, dep.Version)
	}

	return status
}

// GetMissingDependencies returns a list of missing dependencies for a manifest
func (c *DependencyChecker) GetMissingDependencies(ctx context.Context, manifest *domain.PackManifest) ([]domain.PackDependency, error) {
	result, err := c.CheckDependencies(ctx, manifest)
	if err != nil {
		return nil, err
	}

	var missing []domain.PackDependency
	for i, status := range result.Dependencies {
		if status.Missing {
			missing = append(missing, manifest.Dependencies[i])
		}
	}

	return missing, nil
}

// GetUnsatisfiedDependencies returns dependencies that are installed but don't meet version requirements
func (c *DependencyChecker) GetUnsatisfiedDependencies(ctx context.Context, manifest *domain.PackManifest) ([]DependencyStatus, error) {
	result, err := c.CheckDependencies(ctx, manifest)
	if err != nil {
		return nil, err
	}

	var unsatisfied []DependencyStatus
	for _, status := range result.Dependencies {
		if !status.Satisfied && !status.Missing {
			unsatisfied = append(unsatisfied, status)
		}
	}

	return unsatisfied, nil
}

// ValidateInstallation checks dependencies after a pack is installed
// Logs warnings but allows installation to proceed
func (c *DependencyChecker) ValidateInstallation(ctx context.Context, packName string) (*DependencyCheckResult, error) {
	manifest, err := c.manager.GetPackManifest(ctx, packName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pack manifest: %w", err)
	}

	return c.CheckDependencies(ctx, manifest)
}

// BuildDependencyTree builds a tree of all dependencies for a pack
func (c *DependencyChecker) BuildDependencyTree(ctx context.Context, packName string, visited map[string]bool) ([]string, error) {
	if visited == nil {
		visited = make(map[string]bool)
	}

	// Prevent circular dependencies
	if visited[packName] {
		return nil, nil
	}
	visited[packName] = true

	manifest, err := c.manager.GetPackManifest(ctx, packName)
	if err != nil {
		return nil, err
	}

	var allDeps []string
	for _, dep := range manifest.Dependencies {
		allDeps = append(allDeps, dep.Name)

		// Recursively get dependencies of dependencies
		if c.manager.IsInstalled(dep.Name) {
			subDeps, err := c.BuildDependencyTree(ctx, dep.Name, visited)
			if err != nil {
				continue // Skip on error
			}
			allDeps = append(allDeps, subDeps...)
		}
	}

	return allDeps, nil
}

// CheckCircularDependencies checks if adding a pack would create circular dependencies
func (c *DependencyChecker) CheckCircularDependencies(ctx context.Context, manifest *domain.PackManifest) (bool, []string) {
	visited := make(map[string]bool)
	path := []string{manifest.Name}

	return c.detectCycle(ctx, manifest.Name, manifest.Dependencies, visited, path)
}

// detectCycle recursively checks for circular dependencies
func (c *DependencyChecker) detectCycle(ctx context.Context, current string, deps []domain.PackDependency, visited map[string]bool, path []string) (bool, []string) {
	visited[current] = true

	for _, dep := range deps {
		// Check if we've seen this dependency in the current path
		if slices.Contains(path, dep.Name) {
			return true, append(path, dep.Name)
		}

		// If the dependency is installed, check its dependencies
		if c.manager.IsInstalled(dep.Name) {
			depManifest, err := c.manager.GetPackManifest(ctx, dep.Name)
			if err != nil {
				continue
			}

			newPath := append(path, dep.Name)
			if hasCycle, cyclePath := c.detectCycle(ctx, dep.Name, depManifest.Dependencies, visited, newPath); hasCycle {
				return true, cyclePath
			}
		}
	}

	return false, nil
}
