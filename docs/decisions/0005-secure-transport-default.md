# ADR 0005: Noise is the production default

- Status: accepted for bootstrap
- Date: 2026-07-16

## Context

Factory control traffic can reveal operational data and issue physical commands. ESPHome supports Noise encryption, while plaintext remains useful for isolated conformance tests.

## Decision

The simplest and zero-value-oriented production configuration requires Noise. Plaintext is available only through an explicitly insecure option intended for controlled development and simulator tests. Legacy password authentication is outside Milestone 1.

## Consequences

Misconfiguration fails closed. Test APIs must keep insecure fixtures unmistakably synthetic. The implementation will rely on established cryptographic components and test downgrade refusal.
