# Implementation Plan: Deployment and Operations

## Overview

This implementation plan establishes a comprehensive CI/CD pipeline using GitHub Actions, production-ready Kubernetes manifests, and operational tooling for the Asset Injector Microservice. Tasks are ordered to build foundational CI components first, then CD pipeline, followed by infrastructure and monitoring configuration.

## Tasks

- [x] 1. Create CI Pipeline with GitHub Actions
  - [x] 1.1 Create main CI workflow file
    - Create `.github/workflows/ci.yml` with test, lint, and security jobs
    - Configure workflow triggers for pull requests and main branch pushes
    - Add Go module caching using `actions/cache`
    - _Requirements: 1.1, 1.2, 1.3, 1.7_

  - [x] 1.2 Configure test job with race detection
    - Add `go test -race -v ./...` command
    - Configure test timeout and parallel execution
    - Add property test execution with 100 iterations minimum
    - _Requirements: 1.1, 1.2_

  - [x] 1.3 Configure coverage reporting
    - Add coverage generation with `go test -coverprofile`
    - Upload coverage to Codecov or similar service
    - Add coverage badge to README
    - _Requirements: 1.6_

  - [x] 1.4 Configure linting job
    - Add golangci-lint action with project configuration
    - Configure lint to use `.golangci.yml` from repository
    - _Requirements: 1.3_

  - [ ]* 1.5 Write workflow validation tests
    - Use actionlint to validate workflow syntax
    - Test workflow locally with `act` tool
    - _Requirements: 1.1, 1.2, 1.3_

- [x] 2. Add Security Scanning to CI
  - [x] 2.1 Add Go vulnerability scanning
    - Add `govulncheck` step to CI workflow
    - Configure to fail on known vulnerabilities
    - _Requirements: 7.3_

  - [x] 2.2 Add secret detection scanning
    - Add gitleaks action for secret detection
    - Configure to scan all commits in PR
    - _Requirements: 7.4_

  - [x] 2.3 Configure security report generation
    - Generate SARIF format security reports
    - Upload to GitHub Security tab
    - _Requirements: 7.5_

- [x] 3. Checkpoint - Verify CI pipeline works
  - [x] Ensure all CI jobs pass on a test PR
  - [x] Verify caching improves build times
  - [x] Confirm security scans detect test vulnerabilities
  - [x] Ask the user if questions arise

- [x] 4. Create CD Pipeline for Docker Builds
  - [x] 4.1 Create CD workflow file
    - Create `.github/workflows/cd.yml` with build and deploy jobs
    - Configure triggers for main branch and releases
    - Set up GitHub Container Registry authentication
    - _Requirements: 2.1, 2.2, 2.4_

  - [x] 4.2 Configure Docker build with multi-stage
    - Use Docker Buildx for efficient builds
    - Tag images with commit SHA for main branch builds
    - Tag images with version for release builds
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 4.3 Add container vulnerability scanning
    - Add Trivy action for container scanning
    - Configure severity threshold (fail on critical/high)
    - Generate scan report as artifact
    - _Requirements: 7.1, 7.2_

  - [x] 4.4 Configure registry push
    - Push to GitHub Container Registry (ghcr.io)
    - Add image signing with cosign (optional)
    - _Requirements: 2.4_

- [-] 5. Enhance Dockerfile for Production
  - [x] 5.1 Add security labels and metadata
    - Add OCI image labels for source, description, licenses
    - Add build-time arguments for version info
    - _Requirements: 4.1, 4.2_

  - [x] 5.2 Add health check instruction
    - Add HEALTHCHECK instruction to Dockerfile
    - Configure appropriate intervals and timeouts
    - _Requirements: 4.4, 4.5_

  - [x] 5.3 Verify non-root user configuration
    - Ensure USER directive uses non-root UID
    - Test container runs without root privileges
    - _Requirements: 4.1_

- [x] 6. Create Deployment Scripts
  - [x] 6.1 Create health check script
    - Create `scripts/health-check.sh` for deployment verification
    - Implement retry logic with configurable timeout
    - Support both HTTP and exit code verification
    - _Requirements: 2.7_

  - [ ]* 6.2 Write property test for health check script
    - **Property 2: Health Check Verification**
    - Test various response scenarios (success, timeout, error)
    - **Validates: Requirements 2.7, 4.4, 4.5**

  - [x] 6.3 Create rollback script
    - Create `scripts/rollback.sh` for deployment rollback
    - Implement Kubernetes rollback commands
    - Add notification hooks for alerting
    - _Requirements: 6.2, 6.3_

  - [ ]* 6.4 Write property test for rollback script
    - **Property 3: Rollback Idempotency**
    - Test rollback behavior is idempotent
    - **Validates: Requirements 6.2**

- [x] 7. Checkpoint - Verify CD pipeline works
  - Ensure Docker builds complete successfully
  - Verify images are pushed to registry
  - Test health check script locally
  - Ask the user if questions arise

- [x] 8. Create Kubernetes Manifests
  - [x] 8.1 Create base deployment manifest
    - Create `deploy/base/deployment.yaml` with pod spec
    - Configure liveness and readiness probes
    - Set resource requests and limits
    - _Requirements: 4.4, 4.5, 4.6, 9.1, 9.2_

  - [x] 8.2 Create service manifest
    - Create `deploy/base/service.yaml` for internal service
    - Configure appropriate port mappings
    - _Requirements: 9.1_

  - [x] 8.3 Create configmap for environment config
    - Create `deploy/base/configmap.yaml` for non-secret config
    - Include log level, cache size, CORS origins
    - _Requirements: 5.3, 5.4, 5.5_

  - [x] 8.4 Create persistent volume claim
    - Create `deploy/base/pvc.yaml` for data directory
    - Configure appropriate storage class and size
    - _Requirements: 5.6, 9.4_

  - [x] 8.5 Create Kustomize base configuration
    - Create `deploy/base/kustomization.yaml`
    - Reference all base manifests
    - _Requirements: 9.1_

- [x] 9. Create Environment Overlays
  - [x] 9.1 Create staging overlay
    - Create `deploy/overlays/staging/kustomization.yaml`
    - Configure staging-specific replicas (1)
    - Set staging environment variables
    - _Requirements: 5.1, 5.4_

  - [x] 9.2 Create production overlay
    - Create `deploy/overlays/production/kustomization.yaml`
    - Configure production replicas (3+)
    - Set production environment variables
    - _Requirements: 5.1, 5.4_

  - [x] 9.3 Create HPA for production
    - Create `deploy/overlays/production/hpa.yaml`
    - Configure CPU-based autoscaling
    - Set min/max replicas
    - _Requirements: 9.3_

  - [x] 9.4 Create network policy
    - Create `deploy/base/networkpolicy.yaml`
    - Restrict ingress to allowed sources
    - Restrict egress to required destinations
    - _Requirements: 9.5_

- [x] 10. Add Deployment Jobs to CD Pipeline
  - [x] 10.1 Add staging deployment job
    - Add deploy-staging job to CD workflow
    - Configure automatic deployment on main branch
    - Run health check after deployment
    - _Requirements: 2.5, 2.7_

  - [x] 10.2 Add production deployment job
    - Add deploy-production job to CD workflow
    - Configure manual approval requirement using GitHub environments
    - Implement rolling update strategy
    - _Requirements: 2.6, 6.1_

  - [x] 10.3 Add rollback on failure
    - Configure automatic rollback on health check failure
    - Add notification step for rollback events
    - _Requirements: 6.2, 6.3_

- [ ] 11. Checkpoint - Verify Kubernetes deployment works
  - Test deployment to staging environment
  - Verify health probes work correctly
  - Test rollback functionality
  - Ask the user if questions arise

- [x] 12. Create Release Pipeline
  - [x] 12.1 Create release workflow
    - Create `.github/workflows/release.yml` triggered on version tags
    - Build release artifacts with version metadata
    - _Requirements: 3.1, 3.2_

  - [ ]* 12.2 Write property test for version parsing
    - **Property 1: Semantic Version Format Validation**
    - Test version string parsing and validation
    - **Validates: Requirements 3.1**

  - [x] 12.3 Add SBOM generation
    - Add syft action for SBOM generation
    - Generate SPDX format SBOM
    - _Requirements: 7.6_

  - [x] 12.4 Add checksum generation
    - Generate SHA256 and SHA512 checksums for artifacts
    - Include checksums in release notes
    - _Requirements: 3.6_

  - [x] 12.5 Configure GitHub release creation
    - Use GitHub's automatic release notes generation
    - Attach artifacts, SBOM, and checksums to release
    - Tag Docker images with version and latest
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

- [x] 13. Add Monitoring Configuration
  - [x] 13.1 Create ServiceMonitor for Prometheus
    - Create `deploy/monitoring/servicemonitor.yaml`
    - Configure scrape interval and endpoints
    - _Requirements: 8.2_

  - [x] 13.2 Create alerting rules
    - Create `deploy/monitoring/alerting-rules.yaml`
    - Add alerts for high error rate, latency, and availability
    - _Requirements: 8.4_

  - [ ] 13.3 Add OpenTelemetry middleware (optional)
    - Add OpenTelemetry Go SDK dependency
    - Create tracing middleware for Fiber
    - Configure trace context propagation
    - _Requirements: 8.6_

  - [ ]* 13.4 Write property test for trace propagation
    - **Property 4: OpenTelemetry Trace Propagation**
    - Test trace context is propagated correctly
    - **Validates: Requirements 8.6**

- [x] 14. Create Docker Compose for Local Development
  - [x] 14.1 Create docker-compose.yml
    - Create `deploy/docker-compose.yml` for local deployment
    - Configure volume mounts for data persistence
    - Include environment variable configuration
    - _Requirements: 9.6_

  - [x] 14.2 Add development overrides
    - Create `deploy/docker-compose.override.yml` for dev settings
    - Configure debug logging and hot reload (if applicable)
    - _Requirements: 9.6_

- [x] 15. Create Operational Documentation
  - [x] 15.1 Create deployment guide
    - Create `docs/deployment.md` with step-by-step instructions
    - Document environment setup and prerequisites
    - Include troubleshooting section
    - _Requirements: 10.1_

  - [x] 15.2 Create runbook for common operations
    - Create `docs/runbook.md` with operational procedures
    - Document scaling, backup, and restore procedures
    - Include incident response procedures
    - _Requirements: 10.2, 10.3, 10.4, 10.5_

  - [x] 15.3 Create rollback documentation
    - Document manual rollback procedures
    - Include verification steps after rollback
    - _Requirements: 10.6_

  - [x] 15.4 Update README with CI/CD badges
    - Add build status badge
    - Add coverage badge
    - Add security scan badge
    - _Requirements: Documentation_

- [ ] 16. Final Checkpoint - Complete pipeline validation
  - Ensure all CI/CD workflows pass
  - Verify end-to-end deployment to staging
  - Test release workflow with a test tag
  - Validate monitoring integration
  - Ask the user if questions arise

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- The implementation uses GitHub Actions for all automation
- Kubernetes manifests use Kustomize for environment management
- All property tests must be tagged with: **Feature: deployment-operations, Property {number}: {property_text}**
