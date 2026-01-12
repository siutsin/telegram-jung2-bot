# Telegram Jung2 Bot - Go/Rust Rewrite Plan with Buck2

## Executive Summary

This document outlines the plan to rewrite the telegram-jung2-bot from Node.js to Go and Rust, using Buck2 as
the build system. The rewrite aims to improve performance, reduce memory footprint, and leverage the type safety and
concurrency capabilities of Go and Rust.

## Current Architecture Analysis

### Technology Stack (Current)

- **Runtime**: Node.js v24
- **Web Framework**: Fastify 5.x
- **AWS Services**: SQS (message queue), DynamoDB (database)
- **Message Queue Consumer**: sqs-consumer with long polling
- **Build**: Webpack 5
- **Testing**: AVA + C8
- **Deployment**: Docker (distroless) + Kubernetes

### Key Components

1. **Fastify HTTP Server** - Webhook receiver on port 3000
2. **SQS Consumer** - Long-polling queue processor (batch size: 10)
3. **Message Handler** - Command parser and validator
4. **Action Router** - Dispatches SQS messages to Rust for decisions
5. **Data Layer** - DynamoDB persistence
6. **Report Generator** - Statistics aggregation and formatting
7. **Config Manager** - Per-chat settings with admin checks
8. **Scheduler** - Off-work stats trigger

### Supported Commands

```
/jungHelp              → Help message
/topTen                → Top 10 chatters (7 days)
/topDiver              → Top 10 silent users (7 days)
/allJung               → All participants stats (if enabled)
/enableAllJung         → Admin: Enable /allJung
/disableAllJung        → Admin: Disable /allJung
/setOffFromWorkTimeUTC → Admin: Set off-work time (UTC HHMM)
```

### Architecture Flow

```
Telegram → Webhook → Queue to SQS → SQS Consumer (long poll)
                               ↓
                        Rust decides action
                               ↓
           Rust executes DynamoDB + renders response
                               ↓
                   Go sends Telegram response
                               ↓
                         Telegram API

EventBridge Scheduler → Queue to SQS (scheduled actions)
```

## Proposed Architecture (Go + Rust + Buck2)

### Technology Stack (Proposed)

#### Core Language Split

- **Go**: Thin networking layer (HTTP server, SQS polling, AWS SDK bindings for SQS, Telegram HTTP client)
- **Rust**: ALL business logic (command parsing, validation, orchestration, statistics, settings, data models, DynamoDB query construction and result processing, formatting)
- **Buck2**: Build system and dependency management

**Architecture Pattern**: Go receives network data → passes to Rust → returns rendered response

**Asynchronous Flow**:

- **Webhook path**: Go performs minimal validation and enqueues the action to SQS.
- **SQS path**: Go polls SQS, Rust selects the action, executes DynamoDB via the AWS Rust SDK, and returns the rendered response for Go to send.
- **Scheduler path**: EventBridge Scheduler enqueues scheduled actions to SQS to keep webhook latency low.

**Reliability**:

- Configure SQS visibility timeout to exceed the longest expected action duration.
- Use DLQ and retry policy for failed actions, with idempotent handlers to tolerate duplicates.

**Schedule Lifecycle**:

- Store `schedule_name` in `CHATID_TABLE` to manage updates and deletions.
- On `/setOffFromWorkTimeUTC`, create or update the schedule to target SQS with a payload including `chat_id`, `action`, and `scheduled_time`.
- On `my_chat_member` removal (bot kicked), delete the schedule and mark the chat inactive.
- Run a daily cleanup job to remove schedules for chats with no activity in 7 days.
- Enforce idempotency: store `last_run_date` per chat/action and skip any scheduled event where `scheduled_date <= last_run_date`.

#### Key Dependencies

- **Go**:
  - HTTP server: `net/http` (standard library)
  - AWS SDK: `github.com/aws/aws-sdk-go-v2` (SQS only)
  - Logging: `log/slog` (standard library)
  - JSON: `encoding/json` (for FFI data transfer)
- **Rust**:
  - Core business logic library with FFI exposure via cbindgen
  - AWS SDK for Rust (DynamoDB)
  - Command parsing and validation
  - Statistics computations and report formatting
  - Workday bitmask operations
  - Time zone conversions
  - Data models (Message, Chat, User)
  - DynamoDB serialization/deserialization (serde)

### Project Structure

```
telegram-jung2-bot/
├── .buckconfig                     # Buck2 configuration
├── toolchains/
│   └── BUCK                        # Toolchain definitions (Go, Rust, CXX)
├── rust/
│   ├── BUCK                        # Rust build rules
│   ├── Cargo.toml                  # Rust metadata (for cbindgen)
│   ├── cbindgen.toml              # C header generation config
│   └── src/
│       ├── lib.rs                 # FFI exports only
│       ├── commands/
│       │   ├── mod.rs
│       │   ├── handler.rs         # Command dispatcher (ALL business logic)
│       │   ├── statistics.rs     # Stats commands (topTen, topDiver, allJung)
│       │   ├── settings.rs       # Admin commands
│       │   └── help.rs           # Help command
│       ├── models/
│       │   ├── mod.rs
│       │   ├── message.rs        # Message struct with validation
│       │   ├── chat.rs           # Chat struct
│       │   └── user.rs           # User struct
│       ├── services/
│       │   ├── mod.rs
│       │   ├── statistics.rs     # Stats aggregation algorithms
│       │   ├── dynamodb.rs       # DynamoDB access via AWS Rust SDK
│       │   ├── workday.rs        # Workday bitmask operations
│       │   └── datetime.rs       # Time zone conversions
│       └── formatters/
│           ├── mod.rs
│           └── report.rs         # Report formatting (3800 char limit)
│   └── tests/
│       └── **/*.rs               # Rust integration tests (separate crates)
├── go/
│   ├── BUCK                        # Go binary build rules
│   ├── main.go                     # Entry point (HTTP + SQS consumer)
│   ├── core/
│   │   ├── BUCK
│   │   ├── bindings.go            # CGO bindings to Rust
│   │   └── bindings_test.go       # go_test target
│   ├── server/
│   │   ├── BUCK
│   │   ├── server.go              # HTTP server setup (thin)
│   │   ├── handlers.go            # Route handlers (pass to Rust)
│   │   ├── server_test.go         # go_test target
│   │   └── handlers_test.go       # go_test target
│   ├── aws/
│   │   ├── BUCK
│   │   ├── sqs.go                 # SQS client wrapper
│   │   └── sqs_test.go            # go_test target
│   ├── telegram/
│   │   ├── BUCK
│   │   ├── client.go              # Telegram HTTP client (thin)
│   │   └── client_test.go         # go_test target
├── docker/
│   ├── Dockerfile.app             # Main application
├── Makefile                        # Build helpers
└── README.md
```

## Implementation Plan

### Phase 1: Foundation & Infrastructure Setup

#### 1.1 Buck2 Configuration

- [x] Review cgorust reference project
- [x] Create `.buckconfig` with cells for root, prelude, toolchains
- [x] Define toolchain configurations:
  - `system_go_toolchain` (Go 1.25.5+)
  - `system_rust_toolchain` (Rust edition 2024)
  - `system_cxx_toolchain` (for CGO)
  - `system_genrule_toolchain` (for cbindgen)

#### 1.2 Rust Core Library Setup

- [ ] Create `rust/BUCK` with:
  - `rust_library` for core functionality
  - `rust_test` targets for unit and integration tests
  - `genrule` for cbindgen header generation
  - `prebuilt_cxx_library` for CGO integration
- [ ] Implement Rust modules:
  - `statistics.rs`: Data aggregation, ranking, report generation
  - `workday.rs`: Bitmask operations (string to binary, binary matching)
  - `datetime.rs`: UTC to local time conversion
- [ ] Expose C-compatible FFI functions with `#[no_mangle]`
- [ ] Configure `cbindgen.toml` for header generation

#### 1.3 Go CGO Bindings

- [ ] Create `go/core/BUCK` with `go_library` depending on `//rust:cbindgen_core`
- [ ] Implement `bindings.go` with CGO imports:
  ```go
  // #cgo LDFLAGS: -lrust_core
  // #include "rust/core.h"
  import "C"
  ```
- [ ] Create Go wrapper functions for Rust FFI

### Phase 2: Core Components Implementation

#### 2.1 DynamoDB Access (Rust)

- [ ] **Module**: `rust/src/services/dynamodb.rs`
- [ ] Implement using AWS SDK for Rust (`aws-sdk-dynamodb`)
- [ ] Tables:
  - **MESSAGE_TABLE**: Store message statistics with 7-day TTL
    - Partition key: `chatId` (int64)
    - Sort key: `dateCreated` (string, ISO format with UTC+8)
    - Attributes: userId, username, firstName, lastName, chatTitle, ttl
  - **CHATID_TABLE**: Store chat metadata and settings
    - Partition key: `chatId` (int64)
    - Attributes: chatTitle, enableAllJung (bool), offTime (string), workday (int), userCount, messageCount
- [ ] Operations:
  - `save_message(chat_id, msg)` - Update both tables atomically
  - `get_rows_by_chat_id(chat_id, days)` - Query with pagination (LastEvaluatedKey)
  - `get_all_group_ids(off_time, weekday)` - Scan with filters
  - `set_off_from_work_time_utc(chat_id, off_time, workday)`
  - `enable_all_jung(chat_id)` / `disable_all_jung(chat_id)`
  - `scale_up()` - Increase read capacity
- [ ] **Tests**: Rust unit tests and integration tests with DynamoDB local or mocks

#### 2.2 Telegram Client (Go)

- [ ] **Package**: `go/telegram`
- [ ] HTTP client using `net/http` with keep-alive
- [ ] Methods:
  - `SendMessage(ctx, chatId, text, options) error`
  - `IsAdmin(ctx, chatId, userId) (bool, error)`
- [ ] Rate limiting: Implement using `golang.org/x/time/rate`
- [ ] **Tests**: Mock HTTP responses using `httptest`

#### 2.3 SQS Integration (Go)

- [ ] **Package**: `go/aws`
- [ ] **Consumer** (`sqs.go`):
  - Long polling with `WaitTimeSeconds: 20`
  - Batch size: 10 messages
  - Message attributes: chatId, chatTitle, userId, action, offTime, workday, timeString
  - Concurrent processing using goroutines
  - Graceful shutdown on SIGTERM (stop poll loop, drain workers, then exit)
- [ ] **Producer** (`sqs.go`):
  - `SendMessage(ctx, attributes, body)` - Queue commands
  - Message attributes mapping
- [ ] **Handler**:
  - Pass message payload to Rust for action selection and DynamoDB execution
  - Send rendered response to Telegram
  - Delete message after successful processing
- [ ] **Tests**: Mock SQS client

#### 2.4 Command Handlers (Rust)

- [ ] **Module**: `rust/src/commands/handler.rs`
- [ ] Parse message entities for commands
- [ ] Validate parameters and admin permissions
- [ ] Route to appropriate handler modules
- [ ] Return rendered response for Go to send
- [ ] **Tests**: Rust unit tests (`#[cfg(test)]`)

#### 2.5 Rust Statistics Engine

- [ ] **Module**: `rust/src/services/statistics.rs`
- [ ] Functions:
  - `normalize_rows(messages: Vec<Message>) -> Vec<UserStat>` - Aggregate by user
  - `rank_by_count(users: Vec<UserStat>) -> Vec<UserStat>` - Sort by message count
  - `rank_by_last_active(users: Vec<UserStat>) -> Vec<UserStat>` - Sort by last seen
  - `generate_report(users: Vec<UserStat>, report_type: ReportType) -> String` - Format output
  - `calculate_percentages(users: &mut Vec<UserStat>)` - Compute message %
- [ ] **FFI exports**: C-compatible wrappers
- [ ] **Tests**: Rust unit tests (`#[cfg(test)]`)

#### 2.6 Rust Workday Helper

- [ ] **Module**: `rust/src/services/workday.rs`
- [ ] Functions:
  - `workday_string_to_binary(workdays: &str) -> i32` - Convert "MON,TUE,WED" to bitmask
  - `is_weekday_match_binary(weekday: &str, binary: i32) -> bool` - Check if day matches
- [ ] Bitmask constants:
  ```rust
  const SUN: i32 = 1;
  const MON: i32 = 2;
  const TUE: i32 = 4;
  const WED: i32 = 8;
  const THU: i32 = 16;
  const FRI: i32 = 32;
  const SAT: i32 = 64;
  ```
- [ ] **FFI exports**: Expose via cbindgen
- [ ] **Tests**: Rust unit tests

### Phase 3: Testing & Migration

#### 3.1 Test Suite Port

- [ ] **DynamoDB tests** (Rust, `rust/src/services/dynamodb.rs` with `#[cfg(test)]` and `rust/tests/*.rs`):
  - `TestBuildExpression` - Message to DynamoDB expression
  - `TestSaveMessage` - Both tables updated
  - `TestGetRowsByChatId` - Pagination handling
  - `TestGetAllGroupIds` - Scan with filters (legacy + new offTime)
  - `TestSetOffFromWorkTimeUTC` - Settings update
- [ ] **Telegram tests** (`go/telegram/client_test.go`):
  - `TestSendMessage` - API call
  - `TestIsAdmin` - Admin check (true/false cases)
- [ ] **SQS tests** (`go/aws/sqs_test.go`):
  - `TestOnEvent` - All action types (junghelp, alljung, topten, topdiver, etc.)
  - `TestSendMessage` - Queue message creation
  - `TestSQSAttributeCasing` - StringValue vs stringValue handling
- [ ] **Statistics tests** (Rust, `rust/src/services/statistics.rs` with `#[cfg(test)]`):
  - `TestTopTen` - Report format validation
  - `TestTopDiver` - Dual ranking (by count, by last active)
  - `TestAllJung` - Enabled/disabled check
  - `TestOffFromWork` - Header formatting
- [ ] **Workday tests** (Rust, `rust/src/services/workday.rs` with `#[cfg(test)]`):
  - `test_workday_string_to_binary` - MON,TUE,WED,THU,FRI = 62
  - `test_is_weekday_match_binary` - Match and non-match cases
- [ ] **Message parsing tests** (Rust, `rust/src/commands/handler.rs` with `#[cfg(test)]`):
  - All command formats
  - Invalid parameters
  - Admin permission checks
  - Edit message handling (204 response)
- [ ] **Rust integration tests** (`rust/tests/*.rs`):
  - End-to-end command flow with stubbed inputs
  - Formatter output compatibility checks
- [ ] **Integration tests**: Full webhook → SQS → DynamoDB flow

#### 3.2 Performance Benchmarks

- [ ] Benchmark current Node.js implementation
- [ ] Benchmark Go/Rust implementation
- [ ] Metrics to compare:
  - Request latency (p50, p95, p99)
  - Memory usage (RSS, heap)
  - CPU usage
  - Throughput (messages/sec)
  - Cold start time (Docker)

#### 3.3 Compatibility Testing

- [ ] Verify all commands work identically
- [ ] Test with actual Telegram webhooks
- [ ] Validate DynamoDB data compatibility
- [ ] Check SQS message format compatibility
- [ ] Test edge cases from existing test suite

### Phase 4: Deployment & Operations

#### 4.1 Docker Images

- [ ] **Dockerfile.app** (multi-stage):
  1. Buck2 builder stage (build Rust + Go)
  2. Runtime stage (distroless with ca-certificates)
  3. Copy binary and configurations
- [ ] **docker-compose.yml** for local development:
  - App container
  - LocalStack (DynamoDB + SQS + EventBridge Scheduler)

#### 4.2 CI/CD Pipeline

- [ ] GitHub Actions workflow:
  - Lint (golangci-lint, clippy)
  - Build (Buck2)
  - Test (buck2 test //... with `go_test` and `rust_test` targets)
  - Coverage (go cover, tarpaulin with a future Buck2 genrule)
  - Docker build and push
- [ ] Renovate configuration for dependency updates

#### 4.3 Environment Configuration

- [ ] Environment variables (same as current):
  ```
  STAGE=dev|prod
  AWS_REGION=eu-west-1
  LOG_LEVEL=debug|info|warn|error
  TELEGRAM_BOT_TOKEN=...
  MESSAGE_TABLE=messages-table
  CHATID_TABLE=chatIds-table
  EVENT_QUEUE_URL=https://sqs...
  SCALE_UP_READ_CAPACITY=1
  OFF_FROM_WORK_URL=http://...
  ```
- [ ] Configuration struct in Go with validation
- [ ] Secrets and AWS credentials:
  - Use External Secrets Operator to sync secrets and mount them as pod environment variables
  - Use IRSA annotations for AWS access from Kubernetes

#### 4.4 Monitoring & Observability

- [ ] Structured logging (slog in Go)
- [ ] Request tracing (context propagation)
- [ ] Metrics:
  - HTTP request duration
  - SQS message processing time
  - DynamoDB query latency
  - Telegram API errors
- [ ] Expose a Go `/metrics` endpoint for Prometheus scraping
- [ ] Run HTTP and metrics as separate servers; use `errgroup` to trigger graceful shutdown if either exits
- [ ] Health checks:
  - `/ping` endpoint
  - SQS consumer liveness
  - DynamoDB connectivity
- [ ] Graceful shutdown:
  - Stop HTTP server and stop accepting new requests
  - Cancel SQS polling context
  - Drain in-flight workers with a bounded timeout
- [ ] Worker resilience:
  - Recover panics in worker goroutines, log, and respawn a replacement
  - Treat repeated worker panics as a fatal error to trigger a clean restart

#### 4.5 Operational Details

- [ ] Rust AWS SDK runtime:
  - Initialise a single Tokio runtime and DynamoDB client once
  - Avoid per-request client creation; share the client safely across calls
- [ ] Idempotency storage:
  - Store `last_run_date` per chat/action in `CHATID_TABLE`
  - Skip processing when `scheduled_date <= last_run_date`, using conditional updates to avoid races
- [ ] Telegram rate limits:
  - Respect 429 responses with backoff and retry, bounded by a maximum retry window
  - Log and drop messages that exceed the retry budget to avoid queue build-up
- [ ] SQS DLQ policy:
  - Configure a DLQ with a max receive count
  - Alarm on DLQ depth and include message metadata for reprocessing
- [ ] Kubernetes probes:
  - Separate readiness and liveness endpoints for HTTP and metrics servers
  - Readiness should fail when the SQS consumer is not running

### Phase 5: Documentation & Handoff

#### 5.1 Documentation

- [ ] **README.md** updates:
  - Prerequisites (Buck2, Go, Rust, cbindgen)
  - Build instructions (`buck2 build //go:app`)
  - Run instructions (`buck2 run //go:app`)
  - Test instructions (`buck2 test //...`)
- [ ] **ARCHITECTURE.md**:
  - Go/Rust component split rationale
  - CGO integration details
  - Performance characteristics
- [ ] **MIGRATION.md**:
  - Differences from Node.js version
  - Deployment steps
  - Rollback procedure

#### 5.2 Code Quality

- [ ] Go code:
  - `golangci-lint` with strict rules
  - 80%+ test coverage
  - Documentation comments
- [ ] Rust code:
  - `clippy` with strict rules
  - 80%+ test coverage
  - Rustdoc comments

## Technical Decisions & Rationale

### Why Go as Thin Networking Layer?

- **AWS SDK maturity**: Official AWS SDK v2 with excellent support
- **HTTP performance**: Standard library is fast and battle-tested
- **Concurrency**: Native goroutines for SQS long polling
- **Deployment simplicity**: Single static binary
- **Minimal overhead**: Fast C FFI calls to Rust

### Why Rust for ALL Business Logic?

- **Performance**: Zero-cost abstractions, no garbage collection
- **Safety**: Memory safety without runtime overhead, no null pointers
- **Type system**: Strong types prevent entire classes of bugs at compile time
- **Pattern matching**: Clean command routing and error handling
- **Testing**: Excellent testing framework with property-based testing
- **FFI**: Excellent C interop via cbindgen for Go integration
- **Domain modeling**: Rich type system for modeling business rules

### Why Buck2?

- **Incremental builds**: Only rebuild changed components
- **Correct dependency tracking**: Automatic detection of Go/Rust deps
- **Cross-language support**: Seamless Go + Rust integration
- **Reproducible builds**: Hermetic build environment
- **Fast**: Parallel builds with remote caching support

### CGO Performance Considerations

- **Minimize CGO calls**: Batch data before crossing FFI boundary
- **Use C-compatible types**: Avoid complex type conversions
- **Benchmark**: Verify CGO overhead is acceptable vs pure Go
- **Alternative**: If CGO is bottleneck, consider gRPC or pure Go reimplementation

## Migration Strategy

### Approach: Big Bang vs Incremental

**Recommendation**: Big Bang (complete rewrite)

- **Rationale**:
  - Small codebase (~10 source files)
  - No database schema changes needed
  - Identical webhook/SQS interfaces
  - Can test in parallel environment

### Rollout Plan

1. **Build Go/Rust version** alongside Node.js version
2. **Deploy to staging** with same DynamoDB/SQS (use separate Kubernetes deployment)
3. **Shadow testing** (send traffic to both, compare outputs)
4. **Blue-green deployment** to production:
  - Deploy new version as "green"
  - Route 10% of traffic to green
  - Monitor metrics (latency, errors, memory)
  - Gradually increase to 100%
  - Keep blue as fallback for 1 week
5. **Decommission** old version after successful migration

### Rollback Plan

- Keep Node.js Docker image and Kubernetes manifests
- If critical issues:
  1. Route all traffic back to blue (Node.js version)
  2. Scale down green (Go/Rust version)
  3. Investigate and fix issues
  4. Redeploy and retry

## Risk Assessment

### High Risk

- **CGO performance**: FFI overhead may negate Rust performance gains
  - **Mitigation**: Benchmark early, have pure Go fallback plan
- **Buck2 learning curve**: Team unfamiliar with Buck2
  - **Mitigation**: Reference cgorust project, Buck2 documentation
- **Compatibility bugs**: Subtle differences in behavior
  - **Mitigation**: Comprehensive test suite, shadow testing

### Medium Risk

- **AWS SDK differences**: v2 API differences from Node.js SDK
  - **Mitigation**: Careful API mapping, integration tests
- **Time zone handling**: Go vs Rust time libraries
  - **Mitigation**: Unit tests for all timezone conversions
- **Docker image size**: Static binary may be larger
  - **Mitigation**: Use distroless, strip symbols

### Low Risk

- **Deployment complexity**: Kubernetes deployment unchanged
- **Data migration**: No schema changes needed
- **Monitoring**: Same CloudWatch metrics available

## Success Criteria

### Functional Requirements

- [ ] All 7 commands work identically to Node.js version
- [ ] All test cases pass (100% of current test coverage)
- [ ] Admin permission checks enforced
- [ ] Off-work scheduling works for all configured chats
- [ ] DynamoDB TTL cleanup works (7 days)

### Performance Requirements

- [ ] Webhook latency < 100ms (p95)
- [ ] SQS message processing < 500ms (p95)
- [ ] Memory usage < 128MB (vs 256MB for Node.js)
- [ ] CPU usage < 0.25 vCPU (vs 0.5 for Node.js)
- [ ] Cold start time < 2s

### Quality Requirements

- [ ] Go code coverage ≥ 80%
- [ ] Rust code coverage ≥ 80%
- [ ] Zero critical bugs in production (1 month)
- [ ] All golangci-lint/clippy checks pass

## Timeline Estimate

### Development

- **Phase 1** (Foundation): 1 week
- **Phase 2** (Core Implementation): 3 weeks
- **Phase 3** (Testing): 1 week
- **Phase 4** (Deployment): 1 week
- **Phase 5** (Documentation): 3 days

**Total**: ~6 weeks for 1 engineer

### Dependencies

- Buck2 installed and working
- Go 1.25.5+ installed
- Rust edition 2024 toolchain installed
- cbindgen installed
- AWS credentials configured
- Access to existing DynamoDB/SQS/Kubernetes infrastructure

## Decisions

1. **CGO vs Pure Go**: CGO.
2. **HTTP Framework**: `net/http` with chi.
3. **Logging**: `slog` with context.
4. **Metrics**: Prometheus.
5. **Local dev**: Run dummy DynamoDB, SQS, and EventBridge in Docker for local dev and E2E testing.
6. **Error tracking**: BetterStack for now, with a pluggable interface to swap providers (for example Sentry or SigNoz). Default SDK settings unless needed.

## Appendix

### A. Reference Projects

- **cgorust**: `/Users/simon/projects/github/siutsin/cgorust`
  - Demonstrates Buck2 Go/Rust CGO integration
  - Simple HTTP server with Rust FFI (add function)
  - Toolchain configuration reference

### B. Key Files to Review (Current Implementation)

- `src/index.js` - Main entry point (Fastify + SQS consumer)
- `src/sqs.js` - Action dispatcher and queue operations
- `src/statistics.js` - Report generation logic
- `src/dynamodb.js` - Data persistence layer
- `src/workdayHelper.js` - Bitmask operations (to port to Rust)

### C. Telegram Bot Commands Reference

- Bot Father configuration: `telegram/bot_father_setcommands.txt`
- Message format examples: `test/stub/*.json`

### D. Testing Strategy

- **Unit tests**: Go `go_test` targets and Rust `#[cfg(test)]` modules alongside source
- **Integration tests**: Rust integration tests in `rust/tests/` and full webhook flow with test doubles
- **E2E tests**: Against LocalStack or dedicated test AWS account
- **Load tests**: Artillery or k6 for performance validation
- **Compatibility tests**: Compare outputs with Node.js version

---

**Document Version**: 1.0
**Last Updated**: 11/01/2026
**Status**: Approved
