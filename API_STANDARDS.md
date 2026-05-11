# API Standards

This project follows the rules below for every HTTP endpoint, whether it is public, CMS-only, or auth-protected.

## Error Handling

- Never return raw SQL errors, GCS errors, GORM errors, stack traces, or internal implementation details to API clients.
- Always translate known failures into stable API responses using `internal/apiresponse`.
- Use these response types consistently:
  - `400 validation_error` for bad input, invalid query params, invalid path params, and business-rule validation failures.
  - `401` only for authentication failures.
  - `404 not_found` when the requested record or media item does not exist.
  - `409 conflict` for duplicate/unique-constraint style failures.
  - `503 service_unavailable` when a required dependency such as the database, JWT secret, or bucket configuration is unavailable.
  - `500 internal_error` for all unexpected failures.
- Unexpected repository, SQL, transaction, storage, or serialization errors must be treated as internal failures even if the underlying error string looks descriptive.
- Validation errors should be meaningful to the caller and phrased in request terms such as `title is required`, `storage_url is required`, or `end_date must be on or after start_date`.

## Logging

- Every real server-side error must be logged.
- Log the underlying error once at the controller boundary, before converting it into the client-safe API response.
- Include at minimum:
  - functional scope or package name
  - HTTP method
  - request path
  - actual underlying error
- Do not log secrets, raw passwords, JWTs, or full base64 payloads.
- Prefer reusable logging/error helpers over repeating `log.Printf` blocks in each controller.

## Reusable Helper

- Shared endpoint error handling should use `internal/httpapi/errors.go`.
- `httpapi.HandleError` is the preferred controller boundary for:
  - logging the actual failure
  - mapping sentinel errors to `404`, `409`, `503`, or `400`
  - falling back to `500 internal_error`
- Package-specific controllers may still define their own validation-match predicates, but the logging and fallback behavior should stay centralized.

## Controller Rules

- Controllers are responsible for:
  - request binding
  - path/query parsing
  - calling service methods
  - mapping service errors to API responses
- Controllers should not expose raw `err.Error()` unless the error is intentionally classified as client-safe validation feedback.
- New controllers should prefer a package-level `write<Domain>Error` helper that delegates to `httpapi.HandleError`.

## Service Rules

- Services may return:
  - sentinel errors such as `ErrStoreUnavailable`, `Err<Event>NotFound`, and `ErrMediaBucketNotConfigured`
  - client-safe validation errors for business rules
  - underlying internal errors for unexpected database or integration failures
- Services should not decide HTTP status codes directly.
- Services should keep validation messages user-readable when they are meant to reach the client.

## Testing Rules

- New API/package work must maintain at least `90%` test coverage for the touched package.
- New endpoint work must include tests for:
  - success path
  - invalid request/body handling
  - invalid path/query handling where applicable
  - not found behavior
  - dependency unavailable behavior
  - unexpected internal failure fallback
  - conflict behavior when applicable
- Shared helpers such as `internal/httpapi` must have direct unit tests.
- A change is not complete until `go test ./...` passes.

## Review Checklist

- No raw SQL or storage errors are returned to clients.
- All server-side errors are logged.
- Known error classes map to the correct response code.
- Auth-only failures still return `401` instead of `500`.
- Tests cover both success and failure branches.
- New packages reuse shared HTTP error/logging helpers instead of inventing one-off patterns.

