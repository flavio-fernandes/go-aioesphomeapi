# ADR 0010: Preserve direct `.local` resolution without a new dependency

- Status: accepted for M1 MGMT compatibility
- Date: 2026-07-17

## Context

MGMT's immutable `esphome-blink.mcl` and conveyor example use ESPHome's normal
`.local` hostnames. The Richard87 client resolves those names with built-in
multicast DNS (mDNS). Delegating only to the operating-system resolver is a
regression on hosts where mDNS is not configured, and the first acceptance
scripts accidentally hid it with private `/etc/hosts` entries.

Adding a general DNS or service-discovery module would increase the dependency
surface paid by MGMT. Relying on a host daemon would also make behavior vary by
installation.

## Decision

- The default dial path recognizes names ending in `.local` and sends one
  bounded IPv4 mDNS A query before TCP and Noise setup.
- The parser accepts matching A and AAAA answers, follows DNS compression with
  strict jump and size bounds, and ignores malformed or unrelated traffic.
- Context cancellation and the caller's dial timeout bound the lookup.
- Normal DNS remains owned by `net`; injected dialers remain supported.
- This is direct hostname resolution only. Service browsing and discovery are
  not part of the core.
- Use only the standard library. Add no runtime or transitive module.
- Simulator acceptance must provide a real multicast answer in a private
  network namespace and must not inject `/etc/hosts`.

## Security consequences

mDNS is unauthenticated and its answer is treated only as routing input. Noise
still authenticates possession of the configured key before application data
is accepted. Deployments should use a unique key per device and may additionally
set the expected ESPHome name. Resolver errors remain redacted at the public
client boundary.

## Compatibility consequences

Both hostname-based MGMT demos exercise the real resolution path without MCL
changes. The library retains its smaller module graph while matching the valid
encrypted behavior of the preserved Richard87 branch.
