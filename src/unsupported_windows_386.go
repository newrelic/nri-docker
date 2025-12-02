//go:build windows && 386

package main

// ============================================================================
// BUILD ERROR: Windows 32-bit (386) is NOT supported
// ============================================================================
//
// This integration only supports Windows 64-bit (amd64).
// Windows 32-bit support was removed as of version 2.0.0.
//
// To build for Windows, use:
//   GOOS=windows GOARCH=amd64 go build ./src
//
// ============================================================================

// This will cause a clear compile-time error
var _ = Windows_32bit_386_is_not_supported_Use_amd64_instead
