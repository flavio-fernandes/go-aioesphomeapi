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
- Support matrix distinguishes known, typed, simulated, hardware, and production evidence
- No private URL, local path, credential, device identifier, or packet capture in the PR
