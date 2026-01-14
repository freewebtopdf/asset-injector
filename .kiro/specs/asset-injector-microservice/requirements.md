# Requirements Document

## Introduction

The Asset Injector Microservice is a high-performance, mission-critical service that acts as a centralized rules engine for a Web-to-PDF pipeline. The system receives target URLs and returns custom CSS/JavaScript payloads to clean up web pages (e.g., hiding cookie banners, removing ads) before they are rendered into PDFs. The service must provide sub-millisecond response times and handle high-throughput concurrent requests while maintaining data consistency and reliability.

## Glossary

- **Asset_Injector**: The main microservice system that processes URL matching requests
- **Rule**: A configuration entity that defines URL patterns and associated CSS/JS payloads
- **Matcher**: The core engine component that performs URL pattern matching and scoring
- **Rule_Repository**: The interface for rule storage and retrieval operations
- **Snapshot**: The persistent JSON file containing all rules data
- **LRU_Cache**: Least Recently Used cache for storing resolved URL matches
- **Pattern_Score**: Calculated value determining rule specificity and priority
- **Resolve_Request**: HTTP request containing a URL to match against rules
- **Web_to_PDF_Pipeline**: The downstream system that consumes Asset Injector responses

## Requirements

### Requirement 1: URL Pattern Matching and Resolution

**User Story:** As a Web-to-PDF pipeline service, I want to send a target URL and receive the most specific matching CSS/JS assets, so that I can clean up web pages before PDF generation.

#### Acceptance Criteria

1. WHEN a resolve request contains a valid URL, THE Asset_Injector SHALL return the single highest-scoring matching rule
2. WHEN multiple rules match the same URL, THE Matcher SHALL calculate specificity scores using the formula: BasePriority + PatternLength
3. WHEN a rule has a manual priority override, THE Matcher SHALL use that value directly instead of the calculated score
4. WHEN no rules match the provided URL, THE Asset_Injector SHALL return an empty response with appropriate status
5. WHEN exact match patterns are evaluated, THE Matcher SHALL assign a base priority of 1000
6. WHEN regex patterns are evaluated, THE Matcher SHALL assign a base priority of 500
7. WHEN wildcard/glob patterns are evaluated, THE Matcher SHALL assign a base priority of 100
8. WHEN two rules have identical scores, THE Matcher SHALL select the rule with the longer pattern length

### Requirement 2: High-Performance Concurrent Processing

**User Story:** As a high-throughput system, I want the Asset Injector to handle thousands of concurrent requests with minimal latency, so that the Web-to-PDF pipeline maintains optimal performance.

#### Acceptance Criteria

1. WHEN multiple resolve requests arrive simultaneously, THE Asset_Injector SHALL process them concurrently using read locks
2. WHEN rule mutations occur, THE Asset_Injector SHALL use exclusive write locks to maintain data consistency
3. WHEN regex patterns are stored, THE Asset_Injector SHALL pre-compile them during rule creation to avoid runtime compilation
4. WHEN the resolve method executes, THE Asset_Injector SHALL minimize memory allocations in the hot path
5. WHEN resolve requests are processed, THE Asset_Injector SHALL complete within sub-millisecond response times
6. WHEN the LRU cache is queried, THE Asset_Injector SHALL return cached results without re-executing matching logic

### Requirement 3: Rule Management and CRUD Operations

**User Story:** As a system administrator, I want to create, read, update, and delete URL matching rules, so that I can configure the Asset Injector behavior for different websites.

#### Acceptance Criteria

1. WHEN creating a new rule without an ID, THE Asset_Injector SHALL generate a UUID automatically
2. WHEN creating a rule with regex pattern, THE Asset_Injector SHALL validate regex compilation and reject invalid patterns
3. WHEN creating a rule with CSS/JS content, THE Asset_Injector SHALL enforce a maximum size limit of 100KB per field
4. WHEN retrieving all rules, THE Asset_Injector SHALL return the complete list of active rules
5. WHEN deleting a rule by ID, THE Asset_Injector SHALL remove it from both memory storage and persistent snapshot
6. WHEN updating an existing rule, THE Asset_Injector SHALL maintain the original creation timestamp
7. WHEN rule operations complete, THE Asset_Injector SHALL trigger an atomic snapshot save to disk

### Requirement 4: Data Persistence and Storage

**User Story:** As a mission-critical service, I want rule data to persist across restarts and be protected against data corruption, so that the Asset Injector maintains configuration integrity.

#### Acceptance Criteria

1. WHEN the service starts up, THE Asset_Injector SHALL load rules from the snapshot.json file if it exists
2. WHEN the snapshot file is missing, THE Asset_Injector SHALL treat it as a fresh installation and create empty storage
3. WHEN saving rule changes, THE Asset_Injector SHALL use atomic writes (temp file → sync → rename) to prevent corruption
4. WHEN maintaining dual indexing, THE Asset_Injector SHALL keep map and slice structures synchronized for O(1) ID lookups and O(N) iteration
5. WHEN the service shuts down gracefully, THE Asset_Injector SHALL force a final snapshot save to ensure data integrity
6. WHEN data directories don't exist, THE Asset_Injector SHALL auto-create them during startup
7. WHEN concurrent access occurs, THE Asset_Injector SHALL use thread-safe mechanisms to protect data structures

### Requirement 5: HTTP API and Transport Layer

**User Story:** As a client system, I want to interact with the Asset Injector through a well-defined REST API with proper error handling and security measures, so that I can integrate reliably.

#### Acceptance Criteria

1. WHEN receiving POST requests to /v1/resolve, THE Asset_Injector SHALL accept JSON with url field and return matching CSS/JS assets
2. WHEN receiving GET requests to /v1/rules, THE Asset_Injector SHALL return all active rules in JSON format
3. WHEN receiving POST requests to /v1/rules, THE Asset_Injector SHALL create or update rules with validation
4. WHEN receiving DELETE requests to /v1/rules/:id, THE Asset_Injector SHALL remove the specified rule
5. WHEN receiving GET requests to /health, THE Asset_Injector SHALL return 200 OK for liveness probes
6. WHEN receiving GET requests to /metrics, THE Asset_Injector SHALL return cache statistics and system metrics
7. WHEN request bodies exceed 1MB, THE Asset_Injector SHALL reject them with 413 Payload Too Large
8. WHEN input contains leading/trailing whitespace, THE Asset_Injector SHALL automatically trim it
9. WHEN validation errors occur, THE Asset_Injector SHALL return 422 status with structured error messages
10. WHEN successful rule creation occurs, THE Asset_Injector SHALL return 201 Created status

### Requirement 6: Security and Protection Measures

**User Story:** As a security-conscious system, I want the Asset Injector to implement proper security headers and input validation, so that it resists common web attacks.

#### Acceptance Criteria

1. WHEN processing requests, THE Asset_Injector SHALL enforce CORS restrictions to allowed origins only
2. WHEN returning responses, THE Asset_Injector SHALL include security headers for HSTS and XSS protection
3. WHEN receiving large payloads, THE Asset_Injector SHALL enforce strict body size limits to prevent DoS attacks
4. WHEN request timeouts occur, THE Asset_Injector SHALL enforce a hard timeout of 2 seconds maximum
5. WHEN panics occur in handlers, THE Asset_Injector SHALL recover gracefully and return 500 status without crashing
6. WHEN generating request IDs, THE Asset_Injector SHALL create unique UUIDs for request tracing

### Requirement 7: Observability and Monitoring

**User Story:** As a DevOps engineer, I want comprehensive logging and metrics from the Asset Injector, so that I can monitor performance and troubleshoot issues effectively.

#### Acceptance Criteria

1. WHEN processing requests, THE Asset_Injector SHALL log structured JSON with request ID, method, path, latency, and status
2. WHEN cache operations occur, THE Asset_Injector SHALL track hit/miss ratios using atomic counters
3. WHEN metrics are requested, THE Asset_Injector SHALL return cache statistics, rule count, and uptime information
4. WHEN errors occur, THE Asset_Injector SHALL log detailed error information with stack traces for panics
5. WHEN the service starts, THE Asset_Injector SHALL log startup configuration and initialization status

### Requirement 8: Configuration and Environment Management

**User Story:** As a deployment engineer, I want flexible configuration options for different environments, so that I can deploy the Asset Injector across development, staging, and production.

#### Acceptance Criteria

1. WHEN starting up, THE Asset_Injector SHALL load configuration from environment variables and .env files
2. WHEN invalid configuration is detected, THE Asset_Injector SHALL fail fast with clear error messages
3. WHEN port numbers are invalid, THE Asset_Injector SHALL refuse to start and log the configuration error
4. WHEN cache size limits are configured, THE Asset_Injector SHALL validate they are positive integers
5. WHEN CORS origins are specified, THE Asset_Injector SHALL parse and validate the origin list format

### Requirement 9: Caching and Performance Optimization

**User Story:** As a performance-critical service, I want intelligent caching of resolve results, so that frequently requested URLs return instantly without re-computation.

#### Acceptance Criteria

1. WHEN resolve requests are processed, THE LRU_Cache SHALL store results with configurable maximum size (default 10,000 items)
2. WHEN cache capacity is exceeded, THE LRU_Cache SHALL evict least recently used entries to prevent memory leaks
3. WHEN cached results exist, THE Asset_Injector SHALL return them immediately with cache_hit: true indicator
4. WHEN cache misses occur, THE Asset_Injector SHALL execute full matching logic and store results for future requests
5. WHEN rules are modified, THE Asset_Injector SHALL invalidate affected cache entries to maintain consistency

### Requirement 10: Graceful Shutdown and Reliability

**User Story:** As a production system operator, I want the Asset Injector to handle shutdown signals gracefully, so that no data is lost during deployments or restarts.

#### Acceptance Criteria

1. WHEN SIGINT or SIGTERM signals are received, THE Asset_Injector SHALL initiate graceful shutdown sequence
2. WHEN shutting down, THE Asset_Injector SHALL stop accepting new HTTP connections
3. WHEN shutdown is in progress, THE Asset_Injector SHALL complete processing of in-flight requests
4. WHEN final shutdown occurs, THE Asset_Injector SHALL force save the current snapshot to disk
5. WHEN shutdown completes, THE Asset_Injector SHALL log successful termination status