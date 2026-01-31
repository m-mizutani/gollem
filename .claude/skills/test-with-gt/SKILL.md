---
name: test-with-gt
description: Write Go test code using the gt library. Use when writing tests, creating test files, or when the user asks to add tests for Go code.
allowed-tools: Read, Bash(go:*)
---

# Writing Tests with gt

When writing Go test code, use the `gt` library for type-safe assertions.

## Step 1: Locate gt documentation

First, find where gt is installed and read its documentation:

```bash
GT_DIR=$(go list -m -f '{{.Dir}}' github.com/m-mizutani/gt 2>/dev/null)
```

If gt is available, read the relevant documentation from `$GT_DIR/docs/`:

- `docs/README.md` - Overview and quick reference
- `docs/types/*.md` - Detailed documentation for each test type
- `docs/patterns/*.md` - Common patterns (method chaining, Required, Describe)

## Step 2: Choose the right test type

| Data Type | Use | NOT |
|-----------|-----|-----|
| `[]T` (slice) | `gt.Array(t, arr).Length(3)` | `gt.Value(t, len(arr)).Equal(3)` |
| `string` | `gt.String(t, s).Contains("x")` | `gt.Bool(t, strings.Contains(s, "x")).True()` |
| `error` | `gt.NoError(t, err)` | `gt.Value(t, err).Nil()` |
| `int`, `float`, etc. | `gt.Number(t, n).Greater(5)` | `gt.Bool(t, n > 5).True()` |
| `map[K]V` | `gt.Map(t, m).HasKey("k")` | manual check with `gt.Bool` |
| `bool` | `gt.Bool(t, b).True()` | `gt.Value(t, b).Equal(true)` |

## Step 3: Apply common patterns

### Use `Required()` for fail-fast

```go
gt.NoError(t, err).Required()  // Stop immediately if error
gt.Value(t, result).NotNil().Required()  // Stop if nil
```

### Use `Describef()` for context

```go
gt.Array(t, users).
    Describef("Users for tenant %s", tenantID).
    Length(5)
```

### Handle function returns with R1/R2/R3

```go
result := gt.R1(parseJSON(input)).NoError(t)
gt.String(t, result.Name).Equal("Alice")
```

## Quick Reference

**Constructors**: `gt.Value`, `gt.Array`, `gt.Map`, `gt.Number`, `gt.String`, `gt.Bool`, `gt.Error`, `gt.NoError`, `gt.File`, `gt.Cast`, `gt.R1/R2/R3`

**Short aliases**: `gt.V`, `gt.A`, `gt.M`, `gt.N`, `gt.S`, `gt.B`, `gt.F`, `gt.C`
