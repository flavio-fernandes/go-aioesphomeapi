# ADR 0009: Run immutable MGMT examples through generic encrypted scenarios

- Status: accepted for M1 compatibility evidence; name-resolution setup superseded by ADR 0010
- Date: 2026-07-17

## Context

Hashing and type-checking MGMT's original `esphome0.mcl` and
`esphome-blink.mcl` files does not prove runtime compatibility. The examples
must run byte-for-byte through a real MGMT process and the library's Native API
path without requiring physical devices.

`esphome0.mcl` intentionally contains the documentation address
`192.168.1.50`. The simulator's public test key must never cause a listener to
be exposed on a host network merely to accommodate that immutable input.

## Decision

- Provide generic basic-I/O and blink simulator scenarios. Their initial
  states require MGMT to issue observable switch and number corrections.
- Expose sanitized accepted/active connection counts through `Device.Stats`
  for deterministic polling, reconnect, and cleanup assertions.
- Run both immutable MCL files inside private Linux user, mount, and network
  namespaces over Noise.
- Keep the Native API simulator listener on loopback. For the hardcoded
  documentation address only, use a raw TCP forwarder inside the private
  network namespace.
- Before allowing non-loopback forwarding, require the child process to prove
  that its Linux network namespace identifier differs from the identifier
  captured by the parent before isolation.
- Record only deterministic textual assertions. Do not retain traffic,
  namespace identifiers, temporary logs, or generated binaries.

The forwarder transports bytes unchanged and does not implement, terminate, or
bypass the Native API protocol. It is a maintainer acceptance tool, not a
library transport or beginner workflow.

## Consequences

Both original MCL examples can become runtime `mgmt` evidence without editing
their source or weakening the simulator's loopback-only server API. Polling and
reconnect policy remain owned and tested by MGMT. The scenarios remain generic
ESPHome fixtures and add no runtime dependency.
