# Error Handling Rules

- **NEVER silently swallow errors.** Every `err` returned from a function call must be either:
  1. Returned/propagated to the caller (with `goerr.Wrap` for context), or
  2. Explicitly handled with a meaningful fallback that is clearly documented with a comment explaining WHY the error is safe to ignore.
- Setting a variable to `nil` or a zero value when an error occurs (e.g., `if err != nil { data = nil }`) is NOT acceptable error handling. This hides bugs and leads to silent data corruption.
- When decoding data (base64, JSON, etc.), always propagate decode errors rather than falling back to raw/empty data.
