# Security policy

## Current status

There is no supported release yet. Security findings in repository policy, designs, or future code are still welcome.

## Reporting a vulnerability

Use GitHub Private Vulnerability Reporting for this repository. Do not open a public issue containing credentials, device identifiers, network addresses, camera media, crash dumps, packet captures, or exploit details.

If private reporting is unavailable, open a public issue that contains only the phrase `Security contact requested`; a maintainer will establish a private channel. Never attach sensitive material to that issue.

## Security invariants

- Noise encryption is the production default.
- Plaintext transport is rejected unless a caller selects an explicitly insecure option.
- Secrets are accepted at runtime and never logged, serialized into diagnostics, or stored by default.
- Authentication and protocol errors use redacted, typed errors.
- Device input is untrusted: lengths, allocation, concurrency, and retry behavior are bounded.
- The simulator has no production credentials and cannot silently select a real-device target.
- Firmware and hardware operations require an explicit target and a human-visible safety check.

See [the threat model](docs/security-threat-model.md) for the complete design baseline.
