# Implementation Plan: Asset Injector Microservice

## Overview

This implementation plan breaks down the Asset Injector Microservice into discrete, incremental coding tasks. Each task builds upon previous work, starting with core domain logic and progressing through storage, API, and integration layers. The plan emphasizes early validation through testing and includes checkpoints to ensure system stability at each major milestone.

## Tasks

- [x] 1. Set up project structure and core domain models
  - Create Standard Go Project Layout directory structure
  - Define Rule struct with validation tags and JSON serialization
  - Define AppError struct for domain error handling
  - Define core interfaces: RuleRepository, PatternMatcher, CacheManager
  - Set up Go modules with required dependencies (Fiber, zerolog, godotenv, validator)
  - _Requirements: All requirements (foundational)_

- [x] 1.1 Write property test for Rule struct validation
  - **Property 10: Automatic ID generation**
  - **Validates: Requirements 3.1**

- [x] 1.2 Write property test for content size limits
  - **Property 12: Content size limits**
  - **Validates: Requirements 3.3**

- [x] 2. Implement pattern matching engine
  - [x] 2.1 Create Matcher struct with RWMutex for thread safety
    - Implement core matching algorithm with specificity scoring
    - Implement base priority assignment (exact: 1000, regex: 500, wildcard: 100)
    - Add support for manual priority overrides
    - Include pre-compiled regex storage and validation
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 1.6, 1.7, 1.8, 2.3_

- [x] 2.2 Write property test for specificity scoring
  - **Property 2: Specificity score calculation**
  - **Validates: Requirements 1.2**

- [x] 2.3 Write property test for base priority assignment
  - **Property 4: Base priority assignment by rule type**
  - **Validates: Requirements 1.5, 1.6, 1.7**

- [x] 2.4 Write property test for highest scoring rule selection
  - **Property 1: Highest scoring rule selection**
  - **Validates: Requirements 1.1**

- [x] 2.5 Write property test for manual priority override
  - **Property 3: Manual priority override precedence**
  - **Validates: Requirements 1.3**

- [x] 2.6 Write property test for tie-breaking logic
  - **Property 5: Tie-breaking by pattern length**
  - **Validates: Requirements 1.8**

- [x] 2.7 Write property test for regex pre-compilation
  - **Property 8: Regex pre-compilation**
  - **Validates: Requirements 2.3**

- [x] 3. Implement in-memory storage with dual indexing
  - [x] 3.1 Create Store struct with map and slice dual indexing
    - Implement thread-safe CRUD operations using RWMutex
    - Ensure map and slice synchronization for all operations
    - Add atomic snapshot persistence with temp file → sync → rename pattern
    - Include startup rule loading from snapshot.json
    - _Requirements: 3.4, 3.5, 3.6, 3.7, 4.1, 4.3, 4.4, 4.6, 4.7_

- [x] 3.2 Write property test for dual index synchronization
  - **Property 18: Dual index synchronization**
  - **Validates: Requirements 4.4**

- [x] 3.3 Write property test for complete rule retrieval
  - **Property 13: Complete rule retrieval**
  - **Validates: Requirements 3.4**

- [x] 3.4 Write property test for rule deletion completeness
  - **Property 14: Rule deletion completeness**
  - **Validates: Requirements 3.5**

- [x] 3.5 Write property test for timestamp preservation
  - **Property 15: Timestamp preservation on update**
  - **Validates: Requirements 3.6**

- [x] 3.6 Write property test for atomic snapshot persistence
  - **Property 16: Atomic snapshot persistence**
  - **Validates: Requirements 3.7, 4.3**

- [x] 3.7 Write property test for startup rule loading
  - **Property 17: Startup rule loading**
  - **Validates: Requirements 4.1**

- [x] 3.8 Write property test for thread-safe concurrent access
  - **Property 19: Thread-safe concurrent access**
  - **Validates: Requirements 4.7**

- [x] 4. Implement LRU cache with atomic counters
  - [x] 4.1 Create LRU cache using doubly-linked list and hashmap
    - Implement bounded cache with configurable maximum size
    - Add atomic counters for hit/miss ratio tracking
    - Include cache invalidation methods for rule changes
    - Ensure thread-safe operations with proper locking
    - _Requirements: 2.6, 9.1, 9.2, 9.3, 9.4, 9.5_

- [x] 4.2 Write property test for LRU cache size limits
  - **Property 38: LRU cache size limits**
  - **Validates: Requirements 9.1, 9.2**

- [x] 4.3 Write property test for cache hit consistency
  - **Property 9: Cache hit consistency**
  - **Validates: Requirements 2.6, 9.3**

- [x] 4.4 Write property test for cache miss handling
  - **Property 39: Cache miss handling**
  - **Validates: Requirements 9.4**

- [x] 4.5 Write property test for cache invalidation
  - **Property 40: Cache invalidation on rule changes**
  - **Validates: Requirements 9.5**

- [x] 5. Checkpoint - Core logic validation
  - Ensure all core domain tests pass
  - Verify pattern matching works correctly with various rule types
  - Confirm storage operations maintain data consistency
  - Ask the user if questions arise

- [x] 6. Implement configuration management
  - [x] 6.1 Create Config struct with environment variable binding
    - Add validation for all configuration fields
    - Implement fail-fast startup validation
    - Support both .env files and environment variables
    - Include default values for all optional settings
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [x] 6.2 Write property test for configuration validation
  - **Property 36: Configuration validation**
  - **Validates: Requirements 8.2, 8.3, 8.4, 8.5**

- [x] 6.3 Write property test for environment configuration loading
  - **Property 37: Environment configuration loading**
  - **Validates: Requirements 8.1**

- [x] 7. Implement HTTP API handlers with Fiber
  - [x] 7.1 Create HTTP handlers for all endpoints
    - Implement POST /v1/resolve with URL matching and caching
    - Implement GET /v1/rules for rule listing
    - Implement POST /v1/rules for rule creation/update with validation
    - Implement DELETE /v1/rules/:id for rule deletion
    - Implement GET /health for liveness probes
    - Implement GET /metrics for system metrics
    - Add comprehensive input validation and error handling
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.9, 5.10_

- [x] 7.2 Write property test for resolve endpoint functionality
  - **Property 20: Resolve endpoint functionality**
  - **Validates: Requirements 5.1**

- [x] 7.3 Write property test for rules listing endpoint
  - **Property 21: Rules listing endpoint**
  - **Validates: Requirements 5.2**

- [x] 7.4 Write property test for rule creation endpoint
  - **Property 22: Rule creation endpoint**
  - **Validates: Requirements 5.3, 5.10**

- [x] 7.5 Write property test for rule deletion endpoint
  - **Property 23: Rule deletion endpoint**
  - **Validates: Requirements 5.4**

- [x] 7.6 Write unit test for health endpoint
  - Test that GET /health always returns 200 OK
  - _Requirements: 5.5_

- [x] 7.7 Write property test for metrics endpoint data
  - **Property 34: Metrics endpoint data**
  - **Validates: Requirements 5.6, 7.3**

- [x] 7.8 Write property test for validation error responses
  - **Property 26: Validation error responses**
  - **Validates: Requirements 5.9**

- [x] 8. Implement Fiber middleware stack
  - [x] 8.1 Create middleware pipeline in correct order
    - RequestID middleware for UUID generation
    - Structured logging middleware with zerolog
    - Panic recovery middleware with stack trace logging
    - Security headers middleware (HSTS, XSS protection)
    - CORS middleware with origin restrictions
    - Body limit middleware (1MB maximum)
    - Request timeout middleware (2 second limit)
    - _Requirements: 5.7, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 7.1, 7.4_

- [x] 8.2 Write property test for body size limit enforcement
  - **Property 24: Body size limit enforcement**
  - **Validates: Requirements 5.7, 6.3**

- [x] 8.3 Write property test for input sanitization
  - **Property 25: Input sanitization**
  - **Validates: Requirements 5.8**

- [x] 8.4 Write property test for CORS enforcement
  - **Property 27: CORS enforcement**
  - **Validates: Requirements 6.1**

- [x] 8.5 Write property test for security headers inclusion
  - **Property 28: Security headers inclusion**
  - **Validates: Requirements 6.2**

- [x] 8.6 Write property test for request timeout enforcement
  - **Property 29: Request timeout enforcement**
  - **Validates: Requirements 6.4**

- [x] 8.7 Write property test for panic recovery
  - **Property 30: Panic recovery**
  - **Validates: Requirements 6.5**

- [x] 8.8 Write property test for unique request ID generation
  - **Property 31: Unique request ID generation**
  - **Validates: Requirements 6.6**

- [x] 8.9 Write property test for structured request logging
  - **Property 32: Structured request logging**
  - **Validates: Requirements 7.1**

- [x] 8.10 Write property test for error logging with details
  - **Property 35: Error logging with details**
  - **Validates: Requirements 7.4**

- [x] 9. Implement concurrency controls
  - [x] 9.1 Add proper locking strategies to all components
    - Ensure resolve operations use RLock for concurrent reads
    - Ensure write operations use exclusive locks
    - Add atomic counter operations for metrics tracking
    - Verify thread safety across all data structures
    - _Requirements: 2.1, 2.2, 7.2_

- [x] 9.2 Write property test for concurrent read access
  - **Property 6: Concurrent read access**
  - **Validates: Requirements 2.1**

- [x] 9.3 Write property test for write operation exclusivity
  - **Property 7: Write operation exclusivity**
  - **Validates: Requirements 2.2**

- [x] 9.4 Write property test for cache metrics tracking
  - **Property 33: Cache metrics tracking**
  - **Validates: Requirements 7.2**

- [x] 10. Checkpoint - API and middleware validation
  - Ensure all HTTP endpoints work correctly
  - Verify middleware pipeline processes requests properly
  - Confirm error handling returns appropriate status codes
  - Test concurrent request processing
  - Ask the user if questions arise

- [x] 11. Implement main application wiring
  - [x] 11.1 Create main.go with dependency injection
    - Wire all components together with proper interfaces
    - Implement graceful shutdown with signal handling
    - Add startup logging and configuration validation
    - Include final snapshot save on shutdown
    - Set up Fiber server with all middleware and routes
    - _Requirements: 4.5, 7.5, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 11.2 Write unit tests for graceful shutdown scenarios
  - Test SIGINT and SIGTERM signal handling
  - Test final snapshot save during shutdown
  - Test connection handling during shutdown
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 11.3 Write unit test for startup logging
  - Test that startup configuration is logged properly
  - _Requirements: 7.5_

- [x] 12. Create build and deployment artifacts
  - [x] 12.1 Create Dockerfile with multi-stage build
    - Use Alpine builder stage for compilation
    - Use scratch runtime for minimal image size
    - Include proper security practices and non-root user
    - _Requirements: Production deployment_

- [x] 12.2 Create Makefile with development commands
  - Add build, run, test, lint, and docker-build targets
  - Include race detection for tests: `go test -race ./...`
  - Add golangci-lint integration for code quality
  - Include swag command for API documentation generation
  - _Requirements: Development workflow_

- [x] 12.3 Generate Swagger/OpenAPI documentation
  - Add Swagger annotations to all HTTP handlers
  - Generate API documentation using swaggo/swag
  - Include request/response examples and error codes
  - _Requirements: API documentation_

- [x] 13. Final integration and validation
  - [x] 13.1 Run comprehensive test suite
    - Execute all unit tests with race detection
    - Run all property-based tests with full iterations
    - Verify test coverage meets quality standards
    - _Requirements: All requirements (validation)_

- [x] 13.2 Performance and load testing
  - Test concurrent request handling under load
  - Verify sub-millisecond response times for cached requests
  - Confirm memory usage stays within bounds
  - _Requirements: 2.5, performance requirements_

- [x] 14. Final checkpoint - Complete system validation
  - Ensure all tests pass consistently
  - Verify the system meets all performance requirements
  - Confirm proper error handling and recovery
  - Validate graceful shutdown and data persistence
  - Ask the user if questions arise

## Notes

- All tasks are required for comprehensive development from the start
- Each task references specific requirements for traceability
- Property tests validate universal correctness properties with minimum 100 iterations
- Unit tests validate specific examples and edge cases
- Checkpoints ensure incremental validation and provide opportunities for user feedback
- The implementation follows Standard Go Project Layout for maintainability
- All property tests must be tagged with: **Feature: asset-injector-microservice, Property {number}: {property_text}**