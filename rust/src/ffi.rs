//! FFI exports for integration with Go.
//!
//! This module isolates all C-compatible definitions and unsafe entry points
//! so the rest of the Rust codebase remains clean and idiomatic.

use crate::commands;
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int};

/// FFI response payload returned to Go.
///
/// The `response_text` pointer must be freed by calling `free_string`.
#[repr(C)]
pub struct ActionResult {
    pub status_code: c_int,
    pub chat_id: i64,
    pub response_text: *mut c_char,
}

impl ActionResult {
    fn error() -> Self {
        Self {
            status_code: 500,
            chat_id: 0,
            // Null pointer signals no response payload.
            response_text: std::ptr::null_mut(),
        }
    }
}

/// Processes a webhook payload from Telegram.
///
/// # Safety
///
/// The caller must provide a valid, null-terminated C string.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn process_webhook(payload: *const c_char) -> ActionResult {
    if payload.is_null() {
        return ActionResult::error();
    }

    let payload_str = match unsafe { CStr::from_ptr(payload) }.to_str() {
        Ok(s) => s,
        Err(_) => return ActionResult::error(),
    };

    commands::handler::process_telegram_message(payload_str).unwrap_or_else(|_| ActionResult::error())
}

/// Frees a C string previously allocated by this crate.
///
/// # Safety
///
/// The caller must pass a pointer returned by this crate. Passing other pointers
/// results in undefined behaviour.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe { drop(CString::from_raw(s)) };
    }
}
