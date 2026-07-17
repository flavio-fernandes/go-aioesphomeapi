# ADR 0011: Preserve connection causes and asynchronous close reasons

- Status: accepted for M1 operability
- Date: 2026-07-17

## Context

The first client slice replaced dial, hello, write, and read-loop failures with
generic errors or silence. This made a simple `.local` resolution failure look
like an unexplained MGMT outage and prevented `errors.Is` and `errors.As` from
reaching standard network causes.

An ESPHome target address is necessary operational context. The Noise key is a
secret and must never be included. Noise identity and handshake rejection must
retain their existing fail-closed semantics.

## Decision

- Every replaced synchronous error is wrapped with `%w` and a distinct stage.
- Dial and setup errors name the attempted target. mDNS, Noise, and hello expose
  stable public error categories while retaining lower-level causes.
- Framing and protobuf errors retain both their safety category and underlying
  parser or I/O cause.
- `Client.CloseReason()` reports the first asynchronous termination cause after
  `Done()` closes. Intentional `Close()` records no failure.
- Context cancellation, peer disconnect, callback queue overflow, read, decode,
  and response-write failures are distinguishable.
- Tests use only synthetic targets, errors, transports, and public test keys.

## Consequences

MGMT can keep its reconnect policy while logging actionable failure stages.
Applications can use `errors.Is` and `errors.As` instead of parsing strings.
Operators must redact private target addresses before sharing diagnostics; key
material remains excluded by construction and regression tests.
