//! Core Rust library for telegram-jung2-bot.
//!
//! This crate exposes a C-compatible FFI entry point for Go to call into the
//! Rust business logic. All domain logic stays in Rust; Go only handles I/O.
#![warn(clippy::all)]
#![warn(clippy::pedantic)]
#![warn(clippy::nursery)]

pub mod commands;
pub mod ffi;
