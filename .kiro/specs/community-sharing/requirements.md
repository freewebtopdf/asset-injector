# Requirements Document

## Introduction

The Community Sharing feature transforms the Asset Injector Microservice from a single-user local tool into a community-driven platform where users can share, discover, and contribute URL patterns and injectors via GitHub. This feature replaces the monolithic `snapshot.json` approach with a modular, file-based structure that supports version control, attribution, and collaborative development.

## Glossary

- **Rule_Pack**: A collection of related rules bundled together with metadata (e.g., "Cookie Banner Removers", "Ad Blockers")
- **Rule_File**: An individual YAML/JSON file containing a single rule or small set of related rules
- **Community_Repository**: The GitHub repository where users share and discover rule packs
- **Local_Repository**: The user's local directory containing their rules and imported community rules
- **Rule_Manifest**: A metadata file describing a rule pack's contents, author, version, and dependencies
- **Import_Source**: The origin of a rule (local, community, or custom URL)
- **Rule_Namespace**: A unique identifier prefix to prevent rule ID collisions between different sources

## Requirements

### Requirement 1: File-Based Rule Storage

**User Story:** As a developer, I want rules stored as individual files in a structured directory, so that I can version control them with Git and share them easily.

#### Acceptance Criteria

1. WHEN the service starts, THE Asset_Injector SHALL scan a configurable rules directory for rule files
2. WHEN rule files are found, THE Asset_Injector SHALL load rules from YAML or JSON files with `.rule.yaml`, `.rule.json` extensions
3. WHEN a rule file contains multiple rules, THE Asset_Injector SHALL load all rules from that file
4. WHEN rule files are organized in subdirectories, THE Asset_Injector SHALL recursively scan and load them
5. WHEN a rule file has invalid syntax, THE Asset_Injector SHALL log a warning and skip that file without crashing
6. WHEN rules are created via API, THE Asset_Injector SHALL save them as individual files in the local rules directory
7. WHEN rules are updated via API, THE Asset_Injector SHALL update the corresponding rule file on disk

### Requirement 2: Rule Pack Structure

**User Story:** As a rule author, I want to bundle related rules into packs with metadata, so that users can easily discover and install collections of rules.

#### Acceptance Criteria

1. WHEN creating a rule pack, THE Author SHALL provide a manifest.yaml file with name, description, version, and author fields
2. WHEN a rule pack is loaded, THE Asset_Injector SHALL validate the manifest schema and reject invalid packs
3. WHEN a rule pack contains rules, THE Asset_Injector SHALL prefix rule IDs with the pack namespace to prevent collisions
4. WHEN displaying rule information, THE Asset_Injector SHALL include the source pack name and author attribution
5. WHEN a rule pack specifies dependencies, THE Asset_Injector SHALL warn if dependencies are not installed
6. WHEN a rule pack version changes, THE Asset_Injector SHALL support semantic versioning for compatibility checks

### Requirement 3: Community Repository Integration

**User Story:** As a user, I want to browse and install rule packs from a community GitHub repository, so that I can benefit from community-contributed patterns.

#### Acceptance Criteria

1. WHEN a user requests available packs, THE Asset_Injector SHALL fetch the pack index from the configured community repository
2. WHEN installing a community pack, THE Asset_Injector SHALL download and extract the pack to the local rules directory
3. WHEN a community pack is installed, THE Asset_Injector SHALL record the source URL and version for update tracking
4. WHEN checking for updates, THE Asset_Injector SHALL compare local pack versions with community repository versions
5. WHEN updating a pack, THE Asset_Injector SHALL preserve any local modifications in a separate override directory
6. WHEN the community repository is unavailable, THE Asset_Injector SHALL continue operating with locally cached packs

### Requirement 4: Rule Import and Export

**User Story:** As a user, I want to import rules from URLs and export my rules for sharing, so that I can collaborate with others outside the main repository.

#### Acceptance Criteria

1. WHEN importing from a URL, THE Asset_Injector SHALL fetch and validate the rule file before adding it
2. WHEN importing rules, THE Asset_Injector SHALL detect and handle duplicate rule IDs appropriately
3. WHEN exporting rules, THE Asset_Injector SHALL generate valid YAML files with proper metadata
4. WHEN exporting a rule pack, THE Asset_Injector SHALL create a complete directory structure with manifest
5. WHEN importing from untrusted sources, THE Asset_Injector SHALL validate rule content against security constraints
6. WHEN bulk importing, THE Asset_Injector SHALL provide progress feedback and handle partial failures gracefully

### Requirement 5: Rule Attribution and Metadata

**User Story:** As a rule contributor, I want my rules to include attribution and metadata, so that users know who created them and when they were last updated.

#### Acceptance Criteria

1. WHEN a rule is created, THE Asset_Injector SHALL record the author name if provided
2. WHEN a rule is modified, THE Asset_Injector SHALL update the modification timestamp and optionally the modifier name
3. WHEN displaying rules, THE Asset_Injector SHALL show attribution information including author and source
4. WHEN rules are exported, THE Asset_Injector SHALL include all attribution metadata in the output
5. WHEN importing community rules, THE Asset_Injector SHALL preserve original author attribution
6. WHEN a user modifies a community rule, THE Asset_Injector SHALL track it as a local override with new attribution

### Requirement 6: Rule Conflict Resolution

**User Story:** As a user with multiple rule sources, I want clear conflict resolution when rules overlap, so that I understand which rule takes precedence.

#### Acceptance Criteria

1. WHEN multiple rules have the same ID from different sources, THE Asset_Injector SHALL apply priority: local > override > community
2. WHEN loading rules, THE Asset_Injector SHALL detect and report ID conflicts to the user
3. WHEN a conflict exists, THE Asset_Injector SHALL allow users to explicitly choose which rule to use
4. WHEN rules from different packs match the same URL, THE Asset_Injector SHALL use the standard specificity scoring
5. WHEN a user disables a community rule, THE Asset_Injector SHALL remember this preference across restarts
6. WHEN listing rules, THE Asset_Injector SHALL indicate which rules are overridden or in conflict

### Requirement 7: API Extensions for Community Features

**User Story:** As an API consumer, I want endpoints to manage rule packs and community integration, so that I can build tools around the sharing functionality.

#### Acceptance Criteria

1. WHEN receiving GET requests to /v1/packs, THE Asset_Injector SHALL return all installed rule packs with metadata
2. WHEN receiving POST requests to /v1/packs/install, THE Asset_Injector SHALL install a pack from the specified source
3. WHEN receiving DELETE requests to /v1/packs/:name, THE Asset_Injector SHALL uninstall the specified pack
4. WHEN receiving GET requests to /v1/packs/available, THE Asset_Injector SHALL return available community packs
5. WHEN receiving POST requests to /v1/packs/update, THE Asset_Injector SHALL update specified packs to latest versions
6. WHEN receiving GET requests to /v1/rules/:id/source, THE Asset_Injector SHALL return the rule's origin and attribution
7. WHEN receiving POST requests to /v1/rules/export, THE Asset_Injector SHALL generate downloadable rule pack files

