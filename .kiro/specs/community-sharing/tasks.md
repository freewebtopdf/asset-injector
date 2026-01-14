# Implementation Plan: Community Sharing

## Overview

This implementation plan transforms the Asset Injector from snapshot.json-based storage to a file-based, community-sharing architecture. Tasks are ordered to build foundational components first, then layer on community features, and finally add API endpoints.

## Tasks

- [x] 1. Extend domain models for community sharing
  - [x] 1.1 Add attribution and source fields to Rule struct
    - Add Author, ModifiedBy, Description, Tags fields to `internal/domain/rule.go`
    - Add RuleSource struct with Type, PackName, PackVersion, SourceURL fields
    - Add FilePath field for tracking rule file location
    - _Requirements: 5.1, 5.2, 5.3_

  - [x] 1.2 Create pack-related domain types
    - Create `internal/domain/pack.go` with PackInfo, PackManifest, PackUpdate structs
    - Add PackIndex struct for community repository index
    - Add SourceType constants (local, community, override)
    - _Requirements: 2.1, 2.3, 3.3_

  - [x] 1.3 Create load and export types
    - Add LoadError struct for file loading errors
    - Add RuleChangeEvent struct for file system changes
    - Add ExportOptions struct for pack export configuration
    - Add ConflictInfo struct for rule conflict reporting
    - _Requirements: 1.5, 4.6, 6.2_

  - [ ]* 1.4 Write property test for Rule round-trip consistency
    - **Property 1: Rule file round-trip consistency**
    - **Validates: Requirements 1.6, 1.7**

- [x] 2. Implement file-based rule loader
  - [x] 2.1 Create directory scanner component
    - Create `internal/loader/scanner.go` with recursive directory scanning
    - Implement file extension filtering for `.rule.yaml` and `.rule.json`
    - Support configurable root directories (local, community, override)
    - _Requirements: 1.1, 1.2, 1.4_

  - [x] 2.2 Implement YAML/JSON rule parser
    - Create `internal/loader/parser.go` for parsing rule files
    - Support single-rule and multi-rule file formats
    - Handle parsing errors gracefully with detailed error messages
    - _Requirements: 1.2, 1.3, 1.5_

  - [x] 2.3 Implement rule file writer
    - Create `internal/loader/writer.go` for saving rules to disk
    - Support atomic writes using temp file → sync → rename pattern
    - Generate proper YAML format with metadata fields
    - _Requirements: 1.6, 1.7_

  - [x] 2.4 Create RuleLoader interface implementation
    - Create `internal/loader/loader.go` implementing RuleLoader interface
    - Combine scanner, parser, and validator components
    - Support LoadAll, Reload operations
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

  - [ ]* 2.5 Write property tests for file loading
    - **Property 2: File extension filtering**
    - **Property 3: Multi-rule file loading**
    - **Property 4: Recursive directory scanning**
    - **Property 5: Invalid file resilience**
    - **Validates: Requirements 1.2, 1.3, 1.4, 1.5**

- [x] 3. Checkpoint - Verify file-based loading works
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Implement pack management
  - [x] 4.1 Create manifest parser and validator
    - Create `internal/pack/manifest.go` for parsing manifest.yaml files
    - Validate required fields (name, version, description, author)
    - Support semantic version parsing and comparison
    - _Requirements: 2.1, 2.2, 2.6_

  - [x] 4.2 Implement namespace prefixing
    - Create `internal/pack/namespace.go` for rule ID prefixing
    - Apply pack namespace to all rules loaded from community packs
    - Handle namespace stripping for display purposes
    - _Requirements: 2.3_

  - [x] 4.3 Create PackManager implementation
    - Create `internal/pack/manager.go` implementing PackManager interface
    - Implement ListInstalled, Install, Uninstall, Update methods
    - Track installed pack metadata in .source.json files
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

  - [x] 4.4 Implement dependency checking
    - Add dependency validation to pack installation
    - Warn when dependencies are missing but continue loading
    - _Requirements: 2.5_

  - [ ]* 4.5 Write property tests for pack management
    - **Property 6: Manifest validation**
    - **Property 7: Namespace prefixing**
    - **Property 8: Semantic version comparison**
    - **Property 9: Source tracking persistence**
    - **Property 10: Override preservation during updates**
    - **Validates: Requirements 2.1, 2.2, 2.3, 2.6, 3.3, 3.5**

- [ ] 5. Checkpoint - Verify pack management works
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Implement community repository client
  - [x] 6.1 Create GitHub API client
    - Create `internal/community/client.go` implementing CommunityClient interface
    - Implement FetchIndex for retrieving pack index from repository
    - Implement DownloadPack for downloading pack archives
    - _Requirements: 3.1, 3.2_

  - [x] 6.2 Implement pack index caching
    - Cache pack index locally for offline operation
    - Support configurable cache TTL
    - Graceful fallback when repository is unavailable
    - _Requirements: 3.6_

  - [x] 6.3 Implement version checking
    - Create CheckUpdates method to compare local vs remote versions
    - Support GetLatestVersion for individual packs
    - _Requirements: 3.4_

  - [ ]* 6.4 Write unit tests for community client
    - Test index fetching with mock HTTP responses
    - Test offline fallback behavior
    - Test version comparison logic
    - _Requirements: 3.1, 3.4, 3.6_

- [ ] 7. Implement import and export functionality
  - [ ] 7.1 Create rule importer
    - Create `internal/export/importer.go` for importing rules from URLs
    - Validate imported rules against security constraints (size limits)
    - Detect and handle duplicate rule IDs
    - _Requirements: 4.1, 4.2, 4.5_

  - [ ] 7.2 Create rule exporter
    - Create `internal/export/exporter.go` implementing RuleExporter interface
    - Support ExportRule for single rule export
    - Support ExportPack for complete pack generation with manifest
    - _Requirements: 4.3, 4.4_

  - [ ] 7.3 Implement bulk import with progress
    - Support importing multiple rules with progress feedback
    - Handle partial failures gracefully (import valid, report invalid)
    - _Requirements: 4.6_

  - [ ]* 7.4 Write property tests for import/export
    - **Property 11: Export format validity**
    - **Property 12: Duplicate ID detection**
    - **Property 13: Content security validation**
    - **Property 14: Bulk import partial success**
    - **Validates: Requirements 4.2, 4.3, 4.4, 4.5, 4.6**

- [x] 8. Checkpoint - Verify import/export works
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Implement conflict resolution
  - [x] 9.1 Create conflict detector
    - Create `internal/conflict/detector.go` for detecting rule ID conflicts
    - Identify conflicts across local, override, and community sources
    - _Requirements: 6.2_

  - [x] 9.2 Implement priority-based resolution
    - Create `internal/conflict/resolver.go` for applying priority rules
    - Implement local > override > community priority ordering
    - _Requirements: 6.1_

  - [x] 9.3 Implement disabled rules tracking
    - Create `.disabled.json` file management
    - Persist disabled rule preferences across restarts
    - _Requirements: 6.5_

  - [x] 9.4 Integrate conflict info into rule listings
    - Add conflict indicators to rule API responses
    - Show which rules are overridden or in conflict
    - _Requirements: 6.6_

  - [ ]* 9.5 Write property tests for conflict resolution
    - **Property 18: Source priority ordering**
    - **Property 19: Conflict detection and reporting**
    - **Property 20: Disabled rule persistence**
    - **Property 21: Cross-pack URL matching**
    - **Validates: Requirements 6.1, 6.2, 6.4, 6.5, 6.6**

- [x] 10. Checkpoint - Verify conflict resolution works
  - Ensure all tests pass, ask the user if questions arise.

- [-] 11. Implement attribution tracking
  - [x] 11.1 Add attribution to rule creation
    - Update rule creation to record author name if provided
    - Set CreatedAt timestamp on new rules
    - _Requirements: 5.1_

  - [x] 11.2 Add modification tracking
    - Update rule modification to set UpdatedAt and optionally ModifiedBy
    - Track modification history for community rules
    - _Requirements: 5.2_

  - [x] 11.3 Implement override attribution
    - Create override files when community rules are modified locally
    - Preserve original author, add modifier attribution
    - _Requirements: 5.6_

  - [ ]* 11.4 Write property tests for attribution
    - **Property 15: Attribution metadata preservation**
    - **Property 16: Modification tracking**
    - **Property 17: Override attribution**
    - **Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5, 5.6**

- [x] 12. Extend API for community features
  - [x] 12.1 Add pack management endpoints
    - Add GET /v1/packs endpoint for listing installed packs
    - Add POST /v1/packs/install endpoint for installing packs
    - Add DELETE /v1/packs/:name endpoint for uninstalling packs
    - _Requirements: 7.1, 7.2, 7.3_

  - [x] 12.2 Add community discovery endpoints
    - Add GET /v1/packs/available endpoint for listing community packs
    - Add POST /v1/packs/update endpoint for updating packs
    - _Requirements: 7.4, 7.5_

  - [x] 12.3 Add rule source and export endpoints
    - Add GET /v1/rules/:id/source endpoint for rule origin info
    - Add POST /v1/rules/export endpoint for generating pack files
    - _Requirements: 7.6, 7.7_

  - [ ]* 12.4 Write property tests for API endpoints
    - **Property 22: Pack listing completeness**
    - **Property 23: Pack uninstall completeness**
    - **Property 24: Rule source endpoint**
    - **Property 25: Export endpoint functionality**
    - **Validates: Requirements 7.1, 7.3, 7.6, 7.7**

- [x] 13. Update storage layer for file-based operation
  - [x] 13.1 Refactor Store to use RuleLoader
    - Update `internal/storage/store.go` to use file-based loading
    - Remove snapshot.json dependency for new installations
    - Support migration from snapshot.json to file-based storage
    - _Requirements: 1.1, 1.6, 1.7_

  - [x] 13.2 Implement backward compatibility
    - Detect existing snapshot.json and offer migration
    - Support running in legacy mode if needed
    - _Requirements: 1.1_

  - [ ]* 13.3 Write integration tests for storage migration
    - Test migration from snapshot.json to file-based storage
    - Test backward compatibility with existing data
    - _Requirements: 1.1, 1.6, 1.7_

- [x] 14. Update configuration for community features
  - [x] 14.1 Add community configuration options
    - Add CommunityConfig fields to `internal/config/config.go`
    - Support RULES_DIR, LOCAL_RULES_DIR, COMMUNITY_RULES_DIR, OVERRIDE_RULES_DIR
    - Add COMMUNITY_REPO_URL, AUTO_UPDATE_PACKS, WATCH_RULE_FILES options
    - _Requirements: 1.1, 3.1_

  - [ ]* 14.2 Write unit tests for configuration
    - Test configuration loading with various environment variables
    - Test default values for new configuration options
    - _Requirements: 1.1, 3.1_

- [x] 15. Final checkpoint - Full system validation
  - Ensure all tests pass, ask the user if questions arise.
  - Verify all API endpoints work correctly
  - Test end-to-end pack installation and rule loading flow

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The implementation maintains backward compatibility with existing snapshot.json storage 