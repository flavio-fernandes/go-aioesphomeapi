# Security threat model

## Protected assets

- ESPHome Noise pre-shared keys and any legacy credentials used only for compatibility testing
- Device identity, entity names, network topology, addresses, and operational state
- MGMT desired state and command authorization
- Factory availability, controller memory/CPU, and device connection capacity
- Physical safety of motors, actuators, and nearby people or equipment
- Camera images, serial logs, firmware images, and workbench observations

## Trust boundaries

The application, library, network, ESPHome firmware, simulated peer, MGMT adapter, CI runner, and physical workbench are distinct trust zones. A validly encrypted peer is not automatically authorized for every command. The library provides secure transport primitives; the application owns device enrollment and command policy.

## Principal threats and required controls

| Threat | Required control | Verification gate |
|---|---|---|
| Passive or active network interception | Noise enabled by default; plaintext needs `Insecure...` opt-in | integration test proves secure default and downgrade refusal |
| Spoofed `.local` mDNS answer | Treat resolution as untrusted routing input; Noise still authenticates possession of the per-device key; recommend unique keys and expected-name checks | isolated mDNS acceptance plus wrong-key test |
| Secret exposure | runtime-only secret values, target-aware but key-free errors, zero secret fixtures | log/error tests plus secret scan |
| Malformed or hostile peer | frame/message limits, deadlines, bounded queues, panic-free parsing | fuzzing and adversarial simulator scenarios |
| Reconnect storm | jittered bounded backoff, single dial owner per device, circuit state and metrics | deterministic reconnect test and load test |
| Command replay after reconnect | never replay non-idempotent commands implicitly | disconnect-during-command tests |
| Slow consumer memory growth | bounded subscriber queues with documented overflow semantics | race and saturation tests |
| Device impersonation or key reuse | application-managed enrollment; per-device keys recommended | integration guidance and config review |
| Accidental real-device operation | simulator is default in examples; hardware targets explicit | workbench preflight and separate local config |
| Unsafe actuator behavior | local firmware interlocks, comms timeout, max runtime, safe boot, physical e-stop | firmware and physical acceptance checklist |
| Supply-chain compromise | minimal dependencies, pinned CI actions, generated-diff checks, provenance record | dependency review and release gate |
| Dependency-driven MGMT breakage | core module budget, Go-version gate, real MGMT build, module-graph diff | cross-repository compatibility lane |

## Production transport policy

The zero-value or simplest configuration must not create a plaintext production connection. An insecure transport option must be obvious in source review, cannot be selected by an environment-variable typo, and emits a machine-readable warning without revealing an address or secret.

Legacy ESPHome password authentication was removed upstream in the 2026.1 release line. Supporting older password-based devices is not a Milestone 1 requirement and, if ever added, must be isolated behind an explicit legacy package or option with its own removal policy.

## Factory-scale design limits

Configuration will expose finite defaults for maximum frame size, pending commands, subscriber queue depth, devices dialing concurrently, reconnect rate, and per-operation deadlines. Load tests must model hundreds or thousands of simulated devices without using real credentials or network identities.

## Dependency boundary

The core admits no convenience runtime dependency. Protobuf and one established Noise implementation are the only expected M1 candidates, and each needs the evidence in `docs/dependency-policy.md`. The narrow built-in `.local` resolver uses only the standard library; general mDNS discovery modules, CLI, YAML, telemetry, test, MGMT, simulator-framework, and workbench modules remain outside the core graph. Implementing Noise locally is not an acceptable dependency reduction.

## Physical safety boundary

Network software is not the primary safety controller. The ESPHome firmware or dedicated hardware must stop motion on communications timeout, contradictory sensor state, maximum run time, invalid boot state, or physical e-stop. MGMT can request operation and observe state; it must not be the only mechanism capable of stopping a motor.

## Privacy baseline

Fixtures use synthetic hostnames, RFC-reserved documentation addresses, generated keys labeled as test-only, and fictional entity names. Runtime connection errors may name the attempted target but never the Noise key. Redact private targets before sharing diagnostics. Real camera frames, serial numbers, MAC addresses, SSIDs, IPs, paths, usernames, and attachment metadata stay outside version control.
