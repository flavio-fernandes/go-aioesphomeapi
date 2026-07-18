# Protocol sync review checklist

- Upstream repository: `https://github.com/esphome/esphome`
- Canonical path: `esphome/components/api/api.proto`
- Immutable commit and file SHA-256 recorded
- Applicable upstream license and hash recorded
- Compiler, plugins, images, and modules pinned immutably
- Clean generation reproduces the committed tree
- Message IDs remain unique and changes are explained
- Removed/renamed fields and enum values receive compatibility review
- Unknown future values remain panic-free
- Inventory distinguishes client-bound, server-bound, bidirectional, and conditional messages
- Inventory annotations match the locked release/commit and every annotation names a pinned message
- Every generated row has an explicit version/feature gate, MGMT flag, parity class, public behavior, and all evidence levels
- All M1 rows have typed and simulator evidence; empty stronger evidence remains explicit
- Unknown message IDs, enum values, and fields have behavior, status, test plan, and evidence recorded separately
- Support matrix distinguishes known, typed, simulated, hardware, and production evidence
- No private URL, local path, credential, device identifier, or packet capture in the PR
