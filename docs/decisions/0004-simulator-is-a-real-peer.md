# ADR 0004: Simulator implements the device side of the wire protocol

- Status: accepted for bootstrap
- Date: 2026-07-16

## Context

A mock client would test application call patterns but not framing, handshake, subscriptions, reconnect behavior, or protocol evolution.

## Decision

Build an in-process simulated ESPHome peer that shares generated wire definitions and framing with the client. It exposes a deterministic scenario API for entities, state timelines, expected commands, virtual time, and faults.

## Consequences

Application and MGMT tests can use real client paths without hardware. The simulator is not claimed to be ESPHome firmware and must state its fidelity limits. Real-firmware tests remain a separate evidence level.
