# Compatibility Policy

gollem follows [Semantic Versioning](https://semver.org/). This page describes what is guaranteed from v1.0.0 onward, and how changes to interfaces that you implement are released.

## What v1 Guarantees

Within a major version (for example, any v1.x.y release):

- Minor and patch releases do not remove or rename exported packages, types, functions, methods, constants, or variables.
- Minor and patch releases do not change the signature of exported functions and methods, or the method set of exported interfaces.
- Minor releases may add new packages, types, functions, options, struct fields, and methods on concrete types.
- Patch releases fix bugs without adding new API.

As with the [Go 1 compatibility promise](https://go.dev/doc/go1compat), code that relies on the following may break in a minor release:

- Struct literals without field names (`gollem.ToolSpec{"name", "desc", nil}`). Use field names so that new fields do not break your code.
- Embedding a gollem type in your own struct when gollem adds a method or field whose name conflicts with one of yours.

## Changes to Interfaces You Implement

Several exported interfaces are meant to be implemented by your code, including:

- `gollem.Tool` and `gollem.ToolSet`
- `gollem.Strategy`
- `gollem.HistoryRepository`
- `gollem.LLMClient` and `gollem.Session`
- `trace.Handler` and `trace.Repository`
- `planexec.PlanExecuteHooks` and `reflexion.Hooks`

When a new capability requires gollem to ask these implementations for something new (for example, a tool returning an image or a file), gollem changes the interface directly and releases a new major version (`/v2`, `/v3`, ...). gollem does not change these interfaces in a minor or patch release.

After you upgrade to the new major version, the compiler reports every implementation that does not satisfy the changed interface, so you can find each place that needs to support the new capability.

When every implementation must support a new capability, gollem does not use the following approaches instead of a new major version, because each one lets an implementation miss the new capability without a compile error:

- **Optional interfaces detected by type assertion**: the number of interfaces grows with every capability, and a method with a misspelled name or wrong signature is silently ignored, so gollem falls back to the old behavior.
- **A required embedded base struct** that provides default implementations: every implementation must embed it, and a new method that you have not implemented silently uses the default.
- **New fields on request and response structs**: the compiler does not report a new field, so an implementation that ignores it goes unnoticed.

Existing optional interfaces, such as `gollem.ModelNamer`, remain supported. Implementing them is not required, and gollem works without them.

## Serialized History Format

The JSON format of a serialized `gollem.History` is not covered by this policy. The format may change in any release, including a minor release, and each change increments `gollem.HistoryVersion`.

When the JSON is decoded successfully but its `version` field does not match `gollem.HistoryVersion`, `History.UnmarshalJSON` returns an error that wraps `gollem.ErrHistoryVersionMismatch`. This behavior will not change. If the JSON itself cannot be decoded, `History.UnmarshalJSON` returns the JSON decoding error instead. If you store history, check for `gollem.ErrHistoryVersionMismatch` with `errors.Is` and start a new conversation when it matches. See [History](history.md#validate-the-history-version-before-restoring) for an example.
