# Agent Instructions

This document provides directives for LLM agents working on the telegram-jung2-bot Go/Rust rewrite with Buck2.

## Project Context

### What This Project Does

**telegram-jung2-bot** is a Telegram group chat statistics bot that:

- Tracks message counts per user in group chats
- Generates rankings (top chatters, silent users, all participants)
- Supports scheduled off-work statistics reports
- Runs on Kubernetes with DynamoDB and SQS

### Current Mission

**Rewriting from Node.js to Go and Rust with Buck2 build system**

**Goals:**

- Improve performance (50% memory reduction, <100ms latency)
- Leverage type safety and concurrency
- Deploy as a single static binary
- Maintain 100% compatibility with existing data and behaviour

### Reference Projects

**CRITICAL**: Always refer to `~/projects/github/siutsin/cgorust` for CGO/Buck2 patterns

This is the canonical example of Go + Rust + Buck2 integration.

## General Principles

### British English Only

- All code, comments, documentation, and commit messages must use British English.
- Use British English spelling (e.g., "colour", "behaviour", "optimise")
- Format dates as DD/MM/YYYY or YYYY-MM-DD (ISO 8601). Prefer DD Mon YYYY in documentation where clarity matters.

### No Emojis

- Never use emojis in code, comments, documentation, or commit messages
- Keep all communication professional and text-based

## Language Responsibility Split

### Go Owns (Thin Networking Layer Only)

- HTTP server (webhook receiver, route handlers)
- SQS integration (long polling consumer, message sending)
- AWS SDK bindings (SQS operations only)
- Telegram HTTP client (raw HTTP calls only)
- Main entry point (server startup, graceful shutdown)
- **RULE**: Go is a THIN layer - immediately pass data to Rust for processing

### Rust Owns (All Business Logic)

- **Command parsing and validation** (detect commands, validate parameters)
- **Business logic and orchestration** (command handlers, workflow)
- **Statistics engine** (data aggregation, ranking algorithms, report generation)
- **Settings management** (admin checks, configuration validation)
- **Workday operations** (bitmask string-to-int, matching logic)
- **DynamoDB data models** (struct definitions, serialization/deserialization)
- **DynamoDB access logic** (query construction, pagination strategy, result processing)
- **DynamoDB execution** (AWS Rust SDK calls)
- **Telegram message formatting** (report templates, truncation logic)
- **DateTime utilities** (UTC to local timezone conversions)
- **All domain logic** (if it requires thinking, it belongs in Rust)

### CGO Bridge Pattern

```
Telegram Webhook → Go HTTP Handler → Rust Process → DynamoDB (AWS Rust SDK)
                                   ↓
                          Rust Format Response → Go HTTP → Telegram API
```

**RULE**: Go receives network data, passes to Rust, sends the rendered response returned by Rust.

## Critical Architecture Patterns

### Workday Bitmask System

**CRITICAL**: This is a core piece of logic. Never change the bit values.

```rust
// Bitmask representation (DO NOT MODIFY):
const SUN: i32 = 1;   // 0b0000001
const MON: i32 = 2;   // 0b0000010
const TUE: i32 = 4;   // 0b0000100
const WED: i32 = 8;   // 0b0001000
const THU: i32 = 16;  // 0b0010000
const FRI: i32 = 32;  // 0b0100000
const SAT: i32 = 64;  // 0b1000000

// Example: "MON,TUE,WED,THU,FRI" = 2+4+8+16+32 = 62
// Check match: (WEEKDAY_OPTIONS[day] & workdayBinary) !== 0
```

### SQS Message Format Differences

**CRITICAL**: Legacy ECS and Lambda use different attribute key casings.

- **Lambda**: `messageAttributes.action.stringValue` (lowercase 's')
- **Legacy ECS sqs-consumer**: `messageAttributes.action.StringValue` (uppercase 'S')

**RULE**: Always handle both cases with a helper function.

### DynamoDB Schema

**MESSAGE_TABLE:**

- Partition Key: `chatId` (int64)
- Sort Key: `dateCreated` (string, ISO format with UTC+8 offset)
- TTL: 7 days
- Purpose: Per-message statistics

**CHATID_TABLE:**

- Partition Key: `chatId` (int64)
- Attributes: `chatTitle`, `enableAllJung` (bool), `offTime` (string HHMM), `workday` (int bitmask)
- Purpose: Chat metadata and settings

**RULE**: Always handle pagination with `LastEvaluatedKey` in DynamoDB queries.

## Go Style Guide

This document codifies Go best practices for this project.

## References

- [Go by Example](https://gobyexample.com/)
- [Google Go Style Guide](https://google.github.io/styleguide/go/)
- [Effective Go](https://go.dev/doc/effective_go)

## Documentation and Comments

All `.go` files must include extensive comments and docstrings for exported items and non-trivial logic. Use Go doc comments and explain intent, safety, and edge cases in British English.

## Modern Go Syntax

### Use `any` not `interface{}`

```go
// GOOD
func ProcessData(data map[string]any) error

// BAD
func ProcessData(data map[string]interface{}) error
```

### Use `comparable` for generic constraints

```go
// GOOD
func Contains[T comparable](slice []T, item T) bool

// BAD (too restrictive)
func Contains(slice []int, item int) bool
```

## Error Handling

### Rule: Either Bubble Up OR Handle

**Never do both. Never swallow.**

```go
// GOOD: Bubble up
func ReadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file: %w", err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("unmarshal config: %w", err)
    }

    return &cfg, nil
}

// GOOD: Handle (log + fallback)
func LoadConfigWithDefaults(path string) *Config {
    cfg, err := ReadConfig(path)
    if err != nil {
        slog.Warn("failed to load config, using defaults", "error", err, "path", path)
        return DefaultConfig()
    }
    return cfg
}

// BAD: Both bubbling AND handling
func Bad(path string) (*Config, error) {
    cfg, err := ReadConfig(path)
    if err != nil {
        slog.Error("error reading config", "error", err) // Handling
        return nil, err                                   // Also bubbling
    }
}

// BAD: Swallowing error
func Bad2(path string) *Config {
    cfg, err := ReadConfig(path)
    if err != nil {
        // Error is lost!
    }
    return cfg
}

// BAD: Ignoring error
func Bad3(path string) {
    _ = os.WriteFile(path, data, 0644) // Never ignore!
}
```

### Use `fmt.Errorf` with `%w` for wrapping

```go
// GOOD: Preserve error chain
if err != nil {
    return fmt.Errorf("save message to db: %w", err)
}

// BAD: Lose original error
if err != nil {
    return fmt.Errorf("save message to db: %v", err)
}

// BAD: No context
if err != nil {
    return err
}
```

### Check for sentinel errors with `errors.Is`

```go
// GOOD
if errors.Is(err, ErrNotFound) {
    return DefaultValue()
}

// BAD
if err == ErrNotFound {
    // This won't work if err is wrapped
}
```

## Context Usage

### Always pass context as first parameter

```go
// GOOD
func GetMessages(ctx context.Context, chatID int64) ([]Message, error)

// BAD
func GetMessages(chatID int64, ctx context.Context) ([]Message, error)

// BAD
func GetMessages(chatID int64) ([]Message, error)
```

### Use context for cancellation

```go
// GOOD: Respect context cancellation
func ProcessMessages(ctx context.Context, messages []Message) error {
    for _, msg := range messages {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := processMessage(ctx, msg); err != nil {
                return fmt.Errorf("process message %d: %w", msg.ID, err)
            }
        }
    }
    return nil
}

// BAD: Ignores cancellation
func ProcessMessages(ctx context.Context, messages []Message) error {
    for _, msg := range messages {
        if err := processMessage(ctx, msg); err != nil {
            return err
        }
    }
}
```

### Use context for timeouts

```go
// GOOD
func CallExternalAPI(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    // ...
}
```

## Naming Conventions

### Use MixedCaps not underscores

```go
// GOOD
type MessageHandler struct{}
func ParseUserData() {}

// BAD
type Message_Handler struct{}
func parse_user_data() {}
```

### Use short variable names in small scopes

```go
// GOOD: Short names for small scopes
func (h *Handler) ProcessMessage(msg *Message) error {
    // 'msg' is clear in this context
    // 'h' for handler is idiomatic
}

// GOOD: Descriptive names for larger scopes
type TelegramMessageHandler struct {
    sqsClient        *sqs.Client
    telegramAPIToken string
}

// BAD: Too verbose for small scope
func ProcessMessage(telegramMessage *Message) error {
    if telegramMessage == nil {
        return errors.New("nil")
    }
}
```

### Receiver names should be consistent

```go
// GOOD: Use same receiver name throughout type
type Handler struct{}

func (h *Handler) Process() {}
func (h *Handler) Validate() {}
func (h *Handler) Save() {}

// BAD: Inconsistent receiver names
func (h *Handler) Process() {}
func (handler *Handler) Validate() {}
func (this *Handler) Save() {}
```

## Interfaces

### Accept interfaces, return structs

```go
// GOOD
type Storage interface {
    Save(ctx context.Context, msg *Message) error
}

func NewHandler(storage Storage) *Handler {
    return &Handler{storage: storage}
}

// BAD: Returning interface
func NewHandler() Storage {
    return &concreteStorage{}
}
```

### Define interfaces where they're used

```go
// GOOD: Define interface in consumer package
// go/server/handler.go
package server

type MessageStore interface {
    Save(ctx context.Context, msg *Message) error
}

type Handler struct {
    store MessageStore
}

// BAD: Define interface in provider package
// go/aws/store.go
package aws

type MessageStore interface {
    Save(ctx context.Context, msg *Message) error
}

type Store struct{}

func (s *Store) Save(ctx context.Context, msg *Message) error {
    // implementation
}
```

## Struct Tags

### Use consistent struct tag formatting

```go
// GOOD
type Message struct {
    ChatID      int64     `json:"chat_id"`
    UserID      int64     `json:"user_id"`
    Text        string    `json:"text"`
    DateCreated time.Time `json:"date_created"`
}

// BAD: Inconsistent spacing
type Message struct {
    ChatID int64 `json:"chat_id"`
    UserID int64 `json:"user_id"`
}
```

## Concurrency

### Use channels for coordination

```go
// GOOD: Use channels to signal completion
func ProcessBatch(ctx context.Context, items []Item) error {
    results := make(chan error, len(items))

    for _, item := range items {
        go func(item Item) {
            results <- processItem(ctx, item)
        }(item)
    }

    var errs []error
    for i := 0; i < len(items); i++ {
        if err := <-results; err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("batch processing failed: %d errors", len(errs))
    }
    return nil
}
```

### Use sync.WaitGroup for waiting

```go
// GOOD: Use WaitGroup when you don't need results
func NotifyAll(ctx context.Context, clients []Client, msg string) {
    var wg sync.WaitGroup

    for _, client := range clients {
        wg.Add(1)
        go func(c Client) {
            defer wg.Done()
            if err := c.Send(ctx, msg); err != nil {
                slog.Error("failed to notify", "error", err)
            }
        }(client)
    }

    wg.Wait()
}
```

### Use sync.Once for one-time initialization

```go
// GOOD
type Config struct {
    once   sync.Once
    data   *ConfigData
    loadErr error
}

func (c *Config) Load() error {
    c.once.Do(func() {
        c.data, c.loadErr = loadFromFile()
    })
    return c.loadErr
}
```

## Logging with slog

### Use structured logging

```go
// GOOD: Structured logs
slog.Info("processing message",
    "chat_id", msg.ChatID,
    "user_id", msg.UserID,
    "command", msg.Command)

slog.Error("failed to save message",
    "error", err,
    "chat_id", msg.ChatID)

// BAD: Unstructured logs
log.Printf("processing message from chat %d user %d command %s",
    msg.ChatID, msg.UserID, msg.Command)
```

### Use appropriate log levels

```go
slog.Debug("detailed debug info")  // Development only
slog.Info("normal operation")       // Important events
slog.Warn("recoverable issue")      // Degraded but working
slog.Error("operation failed")      // Error that needs attention
```

## Comments

### Document exported items

```go
// GOOD: Document package, types, and functions
// Package telegram provides a thin HTTP client for Telegram Bot API.
package telegram

// Client makes HTTP requests to Telegram Bot API.
type Client struct {
    token string
}

// SendMessage sends a text message to the specified chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
    // Implementation
}

// BAD: No documentation
type Client struct {
    token string
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
}
```

### Use comments to explain "why" not "what"

```go
// GOOD: Explains why
// Use 3800 character limit instead of Telegram's 4096 to account for
// UTF-8 multi-byte characters and avoid truncation in the middle of
// a character sequence.
const MaxMessageLength = 3800

// BAD: States the obvious
// Set max message length to 3800
const MaxMessageLength = 3800
```

## Go Project-Specific Rules

### Use `any` for FFI JSON transfer

```go
// GOOD: Use `any` for flexible JSON marshaling
type RustResponse struct {
    StatusCode int            `json:"status_code"`
    ChatID     int64          `json:"chat_id"`
    Message    string         `json:"message"`
    Metadata   map[string]any `json:"metadata"`
}
```

### Generate mocks with go:generate

```go
// At the top of interface definition files
//go:generate mockgen -destination=client_mock.go -package=telegram github.com/siutsin/telegram-jung2-bot/go/telegram Client

type Client interface {
    SendMessage(ctx context.Context, chatID int64, text string) error
}
```

Then run:
```bash
go generate ./...
```

## Rust Style Guide

This document codifies Rust best practices for this project.

## References

- [The Rust Programming Language Book](https://doc.rust-lang.org/book/)
- [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)
- [Clippy Lints](https://rust-lang.github.io/rust-clippy/)
- [Rustfmt](https://github.com/rust-lang/rustfmt)

## Documentation and Comments

All `.rs` files must include extensive comments and docstrings for public items and non-trivial logic. Use Rustdoc comments (`///` or `//!`) and explain intent, safety, and edge cases in British English.

## Project Structure

### Follow Rust Module Conventions

```
src/
├── lib.rs              # Library root, FFI exports
├── commands/
│   ├── mod.rs          # Re-exports submodules
│   ├── handler.rs
│   ├── statistics.rs
│   ├── settings.rs
│   └── help.rs
├── models/
│   ├── mod.rs
│   ├── message.rs
│   ├── chat.rs
│   └── user.rs
├── services/
│   ├── mod.rs
│   ├── statistics.rs
│   ├── workday.rs
│   └── datetime.rs
└── formatters/
    ├── mod.rs
    └── report.rs
```

### Module Organisation

```rust
// GOOD: lib.rs re-exports modules
pub mod commands;
pub mod models;
pub mod services;
pub mod formatters;

// Re-export commonly used items
pub use models::{Message, Chat, User};
pub use commands::handler::process_telegram_message;

// GOOD: mod.rs in each module
// src/commands/mod.rs
pub mod handler;
pub mod statistics;
pub mod settings;
pub mod help;

// Re-export main items
pub use handler::CommandHandler;
pub use statistics::{TopTenCommand, TopDiverCommand};
```

## Naming Conventions

### Follow Rust Naming Conventions

```rust
// GOOD: snake_case for functions and variables
pub fn process_telegram_message(payload: &str) -> Result<ActionResult>
let user_count = stats.len();

// GOOD: CamelCase for types
pub struct MessageHandler {}
pub enum Command {}
pub trait Repository {}

// GOOD: SCREAMING_SNAKE_CASE for constants
pub const MAX_MESSAGE_LENGTH: usize = 3800;
const WEEKDAY_SUN: i32 = 1;

// BAD: Wrong casing
pub fn ProcessTelegramMessage() {}  // Should be snake_case
pub struct message_handler {}       // Should be CamelCase
const maxMessageLength: usize = 3800; // Should be SCREAMING_SNAKE_CASE
```

### Use Descriptive Names

```rust
// GOOD: Clear, descriptive names
pub fn aggregate_messages_by_user(messages: &[Message]) -> HashMap<UserId, UserStats>

// BAD: Abbreviated or unclear
pub fn agg_msg(m: &[Message]) -> HashMap<i64, UserStats>
```

### Type Suffix for Conversions

```rust
// GOOD: Use clear conversion names
impl Message {
    pub fn from_json(json: &str) -> Result<Self>
    pub fn to_json(&self) -> Result<String>
}

// Also acceptable
impl Message {
    pub fn parse(json: &str) -> Result<Self>
    pub fn serialise(&self) -> Result<String>  // British spelling
}
```

## Error Handling

### Use Result and Option

```rust
// GOOD: Return Result for fallible operations
pub fn parse_workday(input: &str) -> Result<i32, ParseError> {
    if input.is_empty() {
        return Err(ParseError::EmptyInput);
    }

    // Parse logic
    Ok(result)
}

// GOOD: Use Option for optional values
pub fn find_user(users: &[User], id: UserId) -> Option<&User> {
    users.iter().find(|u| u.id == id)
}

// BAD: Panic in library code
pub fn parse_workday(input: &str) -> i32 {
    if input.is_empty() {
        panic!("input cannot be empty");  // Never panic in libraries!
    }
    // ...
}
```

### Custom Error Types with thiserror

```rust
use thiserror::Error;

// GOOD: Descriptive error types
#[derive(Error, Debug)]
pub enum WorkdayError {
    #[error("invalid weekday: {0}")]
    InvalidWeekday(String),

    #[error("empty input provided")]
    EmptyInput,

    #[error("parse error: {0}")]
    ParseError(String),
}

pub fn parse_workday(input: &str) -> Result<i32, WorkdayError> {
    if input.is_empty() {
        return Err(WorkdayError::EmptyInput);
    }

    for day in input.split(',') {
        if !is_valid_weekday(day.trim()) {
            return Err(WorkdayError::InvalidWeekday(day.to_string()));
        }
    }

    Ok(result)
}
```

### Use ? Operator for Propagation

```rust
// GOOD: Use ? for clean error propagation
pub fn process_message(payload: &str) -> Result<ActionResult, ProcessError> {
    let message = Message::from_json(payload)?;
    let command = parse_command(&message)?;
    let result = execute_command(command)?;
    Ok(result)
}

// BAD: Manual matching
pub fn process_message(payload: &str) -> Result<ActionResult, ProcessError> {
    let message = match Message::from_json(payload) {
        Ok(m) => m,
        Err(e) => return Err(e.into()),
    };
    // ...
}
```

### Avoid unwrap() in Production Code

```rust
// GOOD: Handle errors properly
pub fn get_chat_id(message: &Message) -> Result<i64, MessageError> {
    message.chat_id.ok_or(MessageError::MissingChatId)
}

// ACCEPTABLE: unwrap() in tests only
#[cfg(test)]
mod tests {
    #[test]
    fn test_parse_command() {
        let cmd = parse_command("/topTen").unwrap();
        assert_eq!(cmd, Command::TopTen);
    }
}

// BAD: unwrap() in production code
pub fn get_chat_id(message: &Message) -> i64 {
    message.chat_id.unwrap()  // Will panic if None!
}
```

## Ownership and Borrowing

### Prefer Borrowing Over Cloning

```rust
// GOOD: Borrow when possible
pub fn calculate_percentage(stats: &[UserStat], total: i64) -> Vec<f64> {
    stats.iter()
        .map(|stat| (stat.count as f64 / total as f64) * 100.0)
        .collect()
}

// BAD: Unnecessary clone
pub fn calculate_percentage(stats: Vec<UserStat>, total: i64) -> Vec<f64> {
    stats.clone()  // Unnecessary!
        .iter()
        .map(|stat| (stat.count as f64 / total as f64) * 100.0)
        .collect()
}
```

### Use &str for String Parameters

```rust
// GOOD: Accept &str (works with String, &str, &'static str)
pub fn format_username(name: &str) -> String {
    format!("@{}", name)
}

// BAD: Require String (forces caller to allocate)
pub fn format_username(name: String) -> String {
    format!("@{}", name)
}

// GOOD: Return String if ownership is transferred
pub fn generate_report(stats: &[UserStat]) -> String {
    // Build and return owned String
    String::from("Report...")
}
```

### Use Cow for Conditional Allocation

```rust
use std::borrow::Cow;

// GOOD: Use Cow when you might need to allocate
pub fn normalise_username(name: &str) -> Cow<str> {
    if name.starts_with('@') {
        Cow::Borrowed(name)
    } else {
        Cow::Owned(format!("@{}", name))
    }
}
```

## Pattern Matching

### Use match for Exhaustiveness

```rust
// GOOD: Exhaustive match
pub fn handle_command(command: Command, chat_id: i64) -> Result<ActionResult> {
    match command {
        Command::TopTen => statistics::handle_top_ten(chat_id),
        Command::TopDiver => statistics::handle_top_diver(chat_id),
        Command::AllJung => statistics::handle_all_jung(chat_id),
        Command::Help => help::handle_help(chat_id),
        // Compiler ensures all variants are covered
    }
}

// GOOD: Use if let for single pattern
pub fn handle_optional_command(command: Option<Command>) -> Result<()> {
    if let Some(Command::TopTen) = command {
        // Handle TopTen specifically
    }
    Ok(())
}

// BAD: Using unwrap instead of pattern matching
pub fn handle_command(command: Option<Command>) -> Result<ActionResult> {
    let cmd = command.unwrap();  // Don't do this!
    // ...
}
```

### Use _ for Unused Variables

```rust
// GOOD: Prefix with _ or use _ for unused variables
pub fn process_result(result: Result<String, Error>) {
    match result {
        Ok(_value) => println!("success"),  // _value not used
        Err(e) => eprintln!("error: {}", e),
    }
}

// Also acceptable
pub fn process_result(result: Result<String, Error>) {
    match result {
        Ok(_) => println!("success"),  // Don't need value at all
        Err(e) => eprintln!("error: {}", e),
    }
}
```

## Collections and Iterators

### Use Iterators Over Loops

```rust
// GOOD: Iterator chains
pub fn top_n_users(stats: &[UserStat], n: usize) -> Vec<&UserStat> {
    stats.iter()
        .filter(|s| s.message_count > 0)
        .take(n)
        .collect()
}

// GOOD: Functional style
pub fn calculate_total(stats: &[UserStat]) -> i64 {
    stats.iter()
        .map(|s| s.message_count)
        .sum()
}

// ACCEPTABLE: Explicit loop when clearer
pub fn find_user_by_id(stats: &[UserStat], id: UserId) -> Option<&UserStat> {
    for stat in stats {
        if stat.user_id == id {
            return Some(stat);
        }
    }
    None
}
```

### Use collect() with Type Annotations

```rust
// GOOD: Type annotation on variable
pub fn user_ids(stats: &[UserStat]) -> Vec<UserId> {
    let ids: Vec<UserId> = stats.iter()
        .map(|s| s.user_id)
        .collect();
    ids
}

// GOOD: Turbofish syntax
pub fn user_ids(stats: &[UserStat]) -> Vec<UserId> {
    stats.iter()
        .map(|s| s.user_id)
        .collect::<Vec<UserId>>()
}
```

## FFI (Foreign Function Interface)

### Separate FFI from Business Logic

```rust
// GOOD: Pure Rust implementation
pub mod core {
    pub fn workday_string_to_binary(workdays: &str) -> Result<i32, WorkdayError> {
        // Pure Rust logic
        Ok(result)
    }
}

// FFI wrapper in lib.rs
use std::os::raw::{c_char, c_int};
use std::ffi::{CStr, CString};

#[no_mangle]
pub unsafe extern "C" fn workday_string_to_binary(s: *const c_char) -> c_int {
    if s.is_null() {
        return -1;  // Error code
    }

    let c_str = match CStr::from_ptr(s).to_str() {
        Ok(s) => s,
        Err(_) => return -1,
    };

    match core::workday_string_to_binary(c_str) {
        Ok(result) => result,
        Err(_) => -1,
    }
}
```

### Never Panic Across FFI

```rust
// GOOD: Return error codes, never panic
#[no_mangle]
pub unsafe extern "C" fn process_webhook(payload: *const c_char) -> *mut WebhookResult {
    if payload.is_null() {
        return std::ptr::null_mut();
    }

    let payload_str = match CStr::from_ptr(payload).to_str() {
        Ok(s) => s,
        Err(_) => return std::ptr::null_mut(),
    };

    match commands::handler::process_telegram_message(payload_str) {
        Ok(result) => Box::into_raw(Box::new(WebhookResult::from(result))),
        Err(_) => std::ptr::null_mut(),
    }
}

// BAD: Panic across FFI boundary (undefined behaviour!)
#[no_mangle]
pub extern "C" fn bad_function(s: *const c_char) -> c_int {
    let c_str = unsafe { CStr::from_ptr(s) };  // May panic if null!
    let rust_str = c_str.to_str().unwrap();    // May panic!
    42
}
```

### Provide Free Functions

```rust
// GOOD: Provide corresponding free function
#[no_mangle]
pub unsafe extern "C" fn generate_report() -> *mut c_char {
    let report = core::generate_report();
    match CString::new(report) {
        Ok(c_string) => c_string.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

#[no_mangle]
pub unsafe extern "C" fn free_string(s: *mut c_char) {
    if !s.is_null() {
        drop(CString::from_raw(s));
    }
}

#[no_mangle]
pub unsafe extern "C" fn free_webhook_result(result: *mut WebhookResult) {
    if !result.is_null() {
        drop(Box::from_raw(result));
    }
}
```

### Use #[repr(C)] for FFI Structs

```rust
// GOOD: C-compatible struct layout
#[repr(C)]
pub struct WebhookResult {
    status_code: c_int,
    should_save_message: bool,
    message_data: *mut c_char,
    action_data: *mut c_char,
}

// BAD: Default Rust layout (not FFI-safe)
pub struct WebhookResult {
    status_code: i32,
    // ...
}
```

## Documentation

### Document Public Items

```rust
/// Aggregates messages by user ID and calculates statistics.
///
/// # Arguments
///
/// * `messages` - A slice of messages to aggregate
///
/// # Returns
///
/// A vector of user statistics, sorted by user ID
///
/// # Examples
///
/// ```
/// use telegram_jung2_bot::services::statistics::aggregate_by_user;
///
/// let messages = vec![/* ... */];
/// let stats = aggregate_by_user(&messages);
/// ```
pub fn aggregate_by_user(messages: &[Message]) -> Vec<UserStat> {
    // Implementation
}
```

### Use //! for Module Documentation

```rust
//! Statistics service for aggregating and ranking user message data.
//!
//! This module provides functions for:
//! - Aggregating messages by user
//! - Ranking users by message count
//! - Ranking users by last activity
//! - Calculating message percentages

use crate::models::{Message, UserStat};
```

### Document Safety Requirements

```rust
/// Processes a webhook payload from Telegram.
///
/// # Safety
///
/// This function is unsafe because it dereferences a raw pointer.
/// The caller must ensure that:
/// - `payload` is a valid, non-null pointer to a null-terminated C string
/// - The memory pointed to by `payload` is valid for reads
/// - The pointer remains valid for the duration of the function call
#[no_mangle]
pub unsafe extern "C" fn process_webhook(payload: *const c_char) -> *mut WebhookResult {
    // Implementation
}
```

## Performance and Optimisation

### Use &[T] Over &Vec<T>

```rust
// GOOD: Accept slices (more flexible)
pub fn process_messages(messages: &[Message]) -> Vec<UserStat>

// BAD: Require Vec (less flexible)
pub fn process_messages(messages: &Vec<Message>) -> Vec<UserStat>
```

### Avoid Allocations in Hot Paths

```rust
// GOOD: Reuse allocation
pub fn format_reports(stats: &[UserStat]) -> Vec<String> {
    let mut reports = Vec::with_capacity(stats.len());
    for stat in stats {
        reports.push(format_stat(stat));
    }
    reports
}

// GOOD: Iterator instead of allocating intermediate Vec
pub fn user_ids(stats: &[UserStat]) -> impl Iterator<Item = UserId> + '_ {
    stats.iter().map(|s| s.user_id)
}

// BAD: Unnecessary clone in loop
pub fn process_stats(stats: &[UserStat]) {
    for stat in stats {
        let cloned = stat.clone();  // Unnecessary if we only read!
        process(cloned);
    }
}
```

### Use sort_unstable When Order Doesn't Matter

```rust
// GOOD: Unstable sort is faster
pub fn rank_by_count(mut stats: Vec<UserStat>) -> Vec<UserStat> {
    stats.sort_unstable_by(|a, b| b.message_count.cmp(&a.message_count));
    stats
}

// Only use stable sort when you need to preserve relative order
pub fn rank_by_count_stable(mut stats: Vec<UserStat>) -> Vec<UserStat> {
    stats.sort_by(|a, b| b.message_count.cmp(&a.message_count));
    stats
}
```

## Clippy Lints

### Enable Recommended Lints

```rust
// At the top of lib.rs
#![warn(clippy::all)]
#![warn(clippy::pedantic)]
#![warn(clippy::nursery)]
#![allow(clippy::missing_errors_doc)]  // Allow specific lints if needed
```

### Common Clippy Fixes

```rust
// GOOD: Use if let instead of match for single pattern
if let Some(user) = find_user(id) {
    process(user);
}

// BAD: Verbose match
match find_user(id) {
    Some(user) => process(user),
    None => {}
}

// GOOD: Use ok_or instead of match
let value = option.ok_or(Error::NotFound)?;

// BAD: Manual match
let value = match option {
    Some(v) => v,
    None => return Err(Error::NotFound),
};
```

## Rust Project-Specific Rules

### Return Rendered Responses to Go

```rust
// GOOD: Rust executes DynamoDB and returns the rendered response
pub struct ActionResult {
    pub status_code: i32,
    pub chat_id: i64,
    pub response_text: String,
}

pub fn handle_top_ten(chat_id: i64) -> Result<ActionResult, CommandError> {
    let rows = dynamodb::query_last_7_days(chat_id)?;
    let stats = services::statistics::aggregate_by_user(&rows);
    let report = formatters::report::generate_top_ten_report(stats);

    Ok(ActionResult {
        status_code: 200,
        chat_id,
        response_text: report,
    })
}
```

### Use serde for JSON Serialisation

```rust
use serde::{Deserialize, Serialize};

// GOOD: Derive Serialize and Deserialize
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    #[serde(rename = "chatId")]
    pub chat_id: i64,

    #[serde(rename = "userId")]
    pub user_id: i64,

    pub text: String,

    #[serde(rename = "dateCreated")]
    pub date_created: String,
}

impl Message {
    pub fn from_json(json: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(json)
    }

    pub fn to_json(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string(self)
    }
}
```

## Formatting and Linting Commands

### Rustfmt

```bash
# Format all code
cargo fmt

# Check formatting without modifying files
cargo fmt --check
```

### Clippy

```bash
# Run all lints
cargo clippy -- -D warnings

# Run with pedantic lints
cargo clippy -- -D warnings -D clippy::pedantic

# Fix automatically fixable lints
cargo clippy --fix
```

### Running Tests

```bash
# Run all tests
buck2 test //...

# Run specific rust_test target
buck2 test //rust:services_test
```

### Test Coverage

Use tarpaulin for Rust coverage, wired through a Buck2 genrule when available.

## CGO Integration Rules

### Architectural Pattern: Command-Response

**CRITICAL**: Use a command-response pattern where Rust issues instructions to Go.

```rust
// Rust returns the rendered response for Go to send
#[repr(C)]
pub struct ActionResult {
    status_code: c_int,
    chat_id: i64,
    response_text: *mut c_char,
}
```

```go
// Go sends the response returned by Rust
func ProcessWebhook(payload []byte) error {
    result := core.ProcessWebhook(payload)
    if result.StatusCode != http.StatusOK {
        return fmt.Errorf("rust processing failed: %d", result.StatusCode)
    }
    return telegram.SendMessage(ctx, result.ChatID, result.ResponseText)
}
```

### Memory Management

**CRITICAL**: Always free C strings.

```go
// GOOD
func CallRust(data string) string {
    cStr := C.CString(data)
    defer C.free(unsafe.Pointer(cStr))

    resultPtr := C.process_data(cStr)
    defer C.free_string(resultPtr)

    return C.GoString(resultPtr)
}

// BAD: Memory leak!
func Bad(data string) string {
    cStr := C.CString(data)
    return C.GoString(C.process_data(cStr))
}
```

### Data Transfer Pattern

**RULE**: Use JSON for complex data structures across FFI boundary.

```go
// GOOD: JSON for complex structures, proper error handling
type RustResponse struct {
    StatusCode int    `json:"status_code"`
    ChatID     int64  `json:"chat_id"`
    Message    string `json:"message"`
}

func ParseRustResponse(payload string) (RustResponse, error) {
    var resp RustResponse
    if err := json.Unmarshal([]byte(payload), &resp); err != nil {
        return RustResponse{}, fmt.Errorf("unmarshal rust response: %w", err)
    }
    return resp, nil
}

// BAD: Ignoring unmarshal error
func Bad(payload string) RustResponse {
    var resp RustResponse
    _ = json.Unmarshal([]byte(payload), &resp) // Don't ignore errors!
    return resp
}

// BAD: Complex C structs
type ComplexStruct struct {
    // Lots of nested pointers, arrays, etc.
}
```

### Type Safety

**RULE**: Use explicit C types.

```go
// GOOD
func Add(a, b int) int {
    return int(C.add(C.int(a), C.int(b)))
}

// BAD: Implicit conversion
func Add(a, b int) int {
    return int(C.add(a, b)) // May cause issues
}
```

## Buck2 Build Rules

### Toolchain Setup

**CRITICAL**: Always define all required toolchains in `toolchains/BUCK`.

```python
# toolchains/BUCK
load("@prelude//toolchains:cxx.bzl", "system_cxx_toolchain")
load("@prelude//toolchains:go.bzl", "system_go_toolchain", "system_go_bootstrap_toolchain")
load("@prelude//toolchains:rust.bzl", "system_rust_toolchain")
load("@prelude//toolchains:genrule.bzl", "system_genrule_toolchain")

system_genrule_toolchain(name = "genrule", visibility = ["PUBLIC"])
system_cxx_toolchain(name = "cxx", visibility = ["PUBLIC"])
system_rust_toolchain(name = "rust", visibility = ["PUBLIC"])
system_go_bootstrap_toolchain(name = "go_bootstrap", visibility = ["PUBLIC"])
system_go_toolchain(name = "go", visibility = ["PUBLIC"])
```

### Rust Library with FFI

**CRITICAL**: Follow this exact pattern from cgorust reference.

```python
# rust/BUCK
rust_library(
    name = "core",
    srcs = glob(["src/**/*.rs"]),
    crate_root = "src/lib.rs",
    edition = "2024",
    visibility = ["PUBLIC"],
)

genrule(
    name = "cbindgen_headers",
    srcs = glob(["src/**/*.rs"]) + ["cbindgen.toml", "Cargo.toml"],
    out = "core.h",
    bash = "cd $SRCDIR && cbindgen --config cbindgen.toml --output $OUT",
    cmd_exe = "cd %SRCDIR% && cbindgen --config cbindgen.toml --output %OUT%",
    visibility = ["PUBLIC"],
)

prebuilt_cxx_library(
    name = "cbindgen_core",
    static_lib = ":core[staticlib]",
    exported_headers = [":cbindgen_headers"],
    visibility = ["PUBLIC"],
)
```

### Go CGO Library

```python
# go/core/BUCK
go_library(
    name = "core",
    srcs = glob(["*.go"]),
    package_name = "github.com/siutsin/telegram-jung2-bot/go/core",
    deps = ["//rust:cbindgen_core"],
    visibility = ["PUBLIC"],
)
```

### Go Binary

```python
# go/BUCK
go_binary(
    name = "app",
    srcs = glob(["*.go"]),
    deps = [
        "//go/core:core",
        "//go/server:server",
        "//go/aws:aws",
        "//go/telegram:telegram",
    ],
    visibility = ["PUBLIC"],
)
```

## Testing Protocol

### Required Testing

**CRITICAL**: Always run tests after making changes.

#### Build and Test Commands

```bash
# Build main app
buck2 build //go:app

# Run all tests
buck2 test //...

# Run specific test
buck2 test //go/server:server_test

# Run with verbose output
buck2 test //... -v 5
```

### Test Coverage Requirements

- Go code coverage: ≥ 80%
- Rust code coverage: ≥ 80%
- All edge cases from Node.js tests must pass

### Test Organisation

- Go tests live next to source files and run via Buck2 `go_test`.
- Rust unit tests live alongside source with `#[cfg(test)]`; integration tests live in `rust/tests/` and run via Buck2 `rust_test`.

## Error Handling Policy

### Fix, Never Suppress

**CRITICAL**: Always fix the root cause of errors. Never suppress or hide them.

### What to Do

- Investigate the underlying issue
- Apply the proper fix at the source
- Verify the fix resolves the problem

### What NOT to Do

- Do not ignore errors with `_`
- Do not suppress linting rules
- Do not use workarounds that mask the real problem
- Do not panic in FFI code

### Specific Cases

**DynamoDB Pagination:**

Always follow `LastEvaluatedKey` until it is empty. Do not return only the first page of results.

**SQS Message Deletion:**

```go
// GOOD: Delete after successful processing
func ProcessMessage(ctx context.Context, msg *sqs.Message) error {
    if err := handleMessage(msg); err != nil {
        return err // Don't delete on error
    }

    _, err := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
        QueueUrl:      &queueURL,
        ReceiptHandle: msg.ReceiptHandle,
    })
    return err
}

// BAD: Message stays in queue
func Bad(msg *sqs.Message) error {
    handleMessage(msg)
    // Never deleted!
    return nil
}
```

## Automation Behaviour

### Auto-Fix and Auto-Apply

Apply these changes automatically without asking:

- Formatting fixes (gofmt, rustfmt)
- Obvious linting errors
- Adding missing error checks
- Fixing import ordering

### When to Ask

Only ask for confirmation when:

- Significant refactoring required
- Behaviour changes proposed
- Architectural decisions needed
- Deletion of substantial code

## Tool Selection

Use the appropriate tool for each task:

| Task                    | Tool                       | Purpose                       |
|-------------------------|----------------------------|-------------------------------|
| Build Go/Rust           | `buck2 build`              | Incremental builds            |
| Run tests               | `buck2 test`               | Test execution                |
| Generate mocks          | `go generate ./...`        | Generate uber-go/mock mocks   |
| Check cbindgen output   | `buck2 build` + `cat`      | Verify C headers              |
| Go linting              | `golangci-lint`            | Go code quality               |
| Rust linting            | `cargo clippy`             | Rust code quality             |
| Dependency graph        | `buck2 query`              | Show dependencies             |
| Clean build             | `buck2 clean`              | Remove artifacts              |

## Project File Locations

### Current Implementation (Node.js)

Reference these files to understand existing logic:

| File                  | Purpose                                      |
|-----------------------|----------------------------------------------|
| `src/index.js`        | Main entry (Fastify server + SQS consumer)   |
| `src/sqs.js`          | Action dispatcher, queue operations          |
| `src/statistics.js`   | Report generation logic                      |
| `src/dynamodb.js`     | Data persistence layer                       |
| `src/workdayHelper.js`| Bitmask operations (port to Rust)            |
| `src/telegram.js`     | Telegram API client                          |
| `src/settings.js`     | Admin checks, config management              |

### Test Files

| File                       | Purpose                                 |
|----------------------------|-----------------------------------------|
| `test/testDynamodb.js`     | DynamoDB operations tests               |
| `test/testSQS.js`          | SQS message handling tests              |
| `test/testStatistics.js`   | Statistics generation tests             |
| `test/testMessages.js`     | Command parsing tests                   |
| `test/testWorkdayHelper.js`| Bitmask operations tests                |

### Target Structure (Go/Rust)

```
telegram-jung2-bot/
├── .buckconfig
├── toolchains/BUCK
├── rust/
│   ├── BUCK
│   ├── Cargo.toml
│   ├── cbindgen.toml
│   └── src/
│       ├── lib.rs                    # FFI exports
│       ├── commands/
│       │   ├── mod.rs
│       │   ├── handler.rs            # Command dispatcher
│       │   ├── statistics.rs         # topTen, topDiver, allJung
│       │   ├── settings.rs           # Admin commands
│       │   └── help.rs               # Help command
│       ├── models/
│       │   ├── mod.rs
│       │   ├── message.rs            # Message struct
│       │   ├── chat.rs               # Chat struct
│       │   └── user.rs               # User struct
│       ├── services/
│       │   ├── mod.rs
│       │   ├── statistics.rs         # Stats aggregation
│       │   ├── dynamodb.rs           # DynamoDB access via AWS Rust SDK
│       │   ├── workday.rs            # Bitmask operations
│       │   └── datetime.rs           # Time conversions
│       └── formatters/
│           ├── mod.rs
│           └── report.rs             # Report formatting
├── go/
│   ├── BUCK
│   ├── main.go                       # Entry point
│   ├── core/
│   │   ├── BUCK
│   │   ├── bindings.go               # CGO bindings to Rust
│   │   └── bindings_test.go          # Tests next to source
│   ├── server/
│   │   ├── BUCK
│   │   ├── server.go                 # HTTP server setup
│   │   ├── server_test.go            # Tests next to source
│   │   ├── handlers.go               # Route handlers (thin)
│   │   └── handlers_test.go          # Tests next to source
│   ├── aws/
│   │   ├── BUCK
│   │   ├── sqs.go                    # SQS client wrapper
│   │   ├── sqs_test.go               # Tests next to source
│   │   └── sqs_mock.go               # Generated by uber-go/mock
│   └── telegram/
│       ├── BUCK
│       ├── client.go                 # HTTP client for Telegram API
│       ├── client_test.go            # Tests next to source
│       └── client_mock.go            # Generated by uber-go/mock
```

## Quick Reference Commands

### Build

```bash
# Build everything
buck2 build //...

# Build main app
buck2 build //go:app

# Build with verbose output
buck2 build //go:app -v 5
```

### Run

```bash
# Run application
buck2 run //go:app

```

### Test

```bash
# Run all tests
buck2 test //...

# Run specific package tests
buck2 test //go/server:server_test
```

### Debug

```bash
# Check cbindgen output
buck2 build //rust:cbindgen_headers
cat buck-out/v2/gen/root/rust/__cbindgen_headers__/*/core.h

# Show dependency tree
buck2 query "deps(//go:app)"

# Show reverse dependencies
buck2 query "rdeps(//..., //rust:core)"

# Clean build
buck2 clean
```

### Generate and Lint

```bash
# Generate Go mocks (run before tests)
cd go && go generate ./...

# Go linting
cd go && golangci-lint run ./...

# Rust linting
cd rust && cargo clippy -- -D warnings

# Rust formatting check
cd rust && cargo fmt --check
```

## Environment Variables

### Required

```bash
STAGE=dev|prod
AWS_REGION=eu-west-1
TELEGRAM_BOT_TOKEN=xxx
MESSAGE_TABLE=messages-table
CHATID_TABLE=chatIds-table
EVENT_QUEUE_URL=https://sqs...
```

### Optional

```bash
LOG_LEVEL=debug|info|warn|error
SCALE_UP_READ_CAPACITY=1
OFF_FROM_WORK_URL=http://...
```

## Critical Compatibility Requirements

### Data Compatibility

**CRITICAL**: The Go/Rust version must work with existing DynamoDB data.

- Do not change table schemas
- Handle both old and new data formats
- Support legacy default values (UTC 1000 = HKT 1800 for Mon-Fri)

### SQS Message Compatibility

**CRITICAL**: Must handle messages from both Node.js and Go/Rust versions during migration.

- Support both `stringValue` and `StringValue` attribute keys
- Maintain exact message format
- Same action types: `topten`, `alljung`, `topdiver`, etc.

### Telegram API Compatibility

**CRITICAL**: Response format must be identical to Node.js version.

- Same message formatting (headers, footers, emojis)
- Same character limits (3800 UTF-8 characters)
- Same error messages

## Known Gotchas

### CGO Performance

**WARNING**: CGO calls have ~50-200ns overhead. For tight loops, this adds up.

**Mitigation**: Batch operations before crossing FFI boundary.

### DynamoDB Attribute Naming

**WARNING**: DynamoDB uses different key names than JSON.

**Example**: `chatId` in DynamoDB, `chat_id` in JSON.

**Solution**: Always use struct tags.

### Telegram Message Truncation

**WARNING**: Telegram has a 4096 character limit, but we use 3800 for UTF-8 safety.

**Rule**: Always truncate reports at 3800 characters.

### SQS Long Polling

**WARNING**: Long polling with WaitTimeSeconds reduces API calls but increases latency.

**Current Setting**: 20 seconds wait time.

## Success Criteria

### Functional

- All 7 commands work identically to Node.js version
- All test cases pass (100% of current tests)
- Admin permission checks enforced
- Off-work scheduling works for all configured chats

### Performance

- Webhook latency < 100ms (p95)
- SQS message processing < 500ms (p95)
- Memory usage < 128MB
- Cold start time < 2s

### Quality

- Go code coverage ≥ 80%
- Rust code coverage ≥ 80%
- All golangci-lint/clippy checks pass
- Zero critical bugs in production (1 month)

## When in Doubt

1. Check `docs/REWRITE_PLAN.md` for detailed implementation steps
2. Refer to `/Users/simon/projects/github/siutsin/cgorust` for CGO/Buck2 patterns
3. Look at current Node.js implementation in `src/` for logic reference
4. Check test files in `test/` for expected behaviour
5. Ask the user for clarification on architectural decisions
