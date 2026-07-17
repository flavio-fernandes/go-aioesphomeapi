# go-aioesphomeapi

An independent, Go-native client for the [ESPHome Native API](https://developers.esphome.io/architecture/api/protocol_details/), built first to be the safest and smallest library MGMT can use for native ESPHome integration.

> [!IMPORTANT]
> This repository contains architecture and compatibility specifications, not a usable client yet. The [support matrix](docs/support-matrix.md) is the authoritative record of implemented and verified behavior.

## The realigned goal

MGMT's experimental [`feat/esphome`](https://github.com/purpleidea/mgmt/compare/master...flavio-fernandes:mgmt:feat/esphome) branch currently uses [`Richard87/esphome-apiclient`](https://github.com/Richard87/esphome-apiclient). This project will provide the MGMT-required behavior behind a deliberately small compatibility facade, then improve it with secure defaults, bounded concurrency, deterministic device simulation, current protocol tracking, and a conservative dependency budget.

Success means MGMT can replace the client dependency without changing the behavior of its existing `.mcl` examples. The intended migration changes Go import paths and only the smallest reviewed adapter details; it does not rename MCL functions, resources, parameters, or semantics.

This is still a greenfield implementation. The reference client is a behavioral baseline, not a code base. The official ESPHome protocol definition remains wire truth.

## Start here

- Copy/paste repository commands: [cheatsheet](CHEATSHEET.md)
- Exact MGMT behavior we must preserve: [MGMT compatibility contract](docs/mgmt-integration.md)
- What is implemented and evidenced: [support matrix](docs/support-matrix.md)
- Why dependencies face a high bar: [dependency policy](docs/dependency-policy.md)
- Controlled delivery sequence: [roadmap](docs/roadmap.md)
- Exact Milestone 1 build order: [implementation sequence](docs/m1-implementation-plan.md)
- How the reference implementations compare: [baseline audit](docs/reference-baseline.md)

Documentation is part of the product. Runnable commands must be tested, safe by default, and explicit about prerequisites. The [documentation contract](docs/documentation-style.md) applies to every feature.

## Design promises

- Existing MGMT `.mcl` behavior is a release-blocking compatibility contract.
- Core types remain generic ESPHome concepts; MGMT and conveyor types stay outside the library.
- Noise is required by the normal production path. Plaintext requires an unmistakable insecure opt-in.
- One concurrency-safe session per device has bounded queues, observable reconnects, and no silent command replay.
- A deterministic simulated device exercises the real framing and session path without hardware.
- The standard library is preferred. Every runtime dependency needs an ADR and evidence; convenience dependencies do not enter the core.
- Generated protobuf compatibility and the stable handwritten API are clearly separated.
- No credentials, private network data, real device identifiers, camera media, or personal contact data belong in the repository.

## Intended shape

```mermaid
flowchart LR
    MCL["Existing MGMT .mcl"] --> Adapter["MGMT adapter and shared session"]
    Adapter --> Compat["Small MGMT compatibility facade"]
    App["Other Go application"] --> Public["Typed public API"]
    Compat --> Session["Bounded device session"]
    Public --> Session
    Session --> Wire["Pinned Native API wire layer"]
    Wire --> Device["ESPHome device"]
    Wire --> Sim["Deterministic simulated device"]
```

The conveyor demonstration is the first visible acceptance system, not the library architecture. See the [architecture](docs/architecture.md) and [conveyor profile](docs/conveyor-demo.md).

## Repository status

The repository is public and GPL-3.0-only licensed. Gate 0 is being realigned around the immutable MGMT compatibility baseline recorded in [`compatibility/mgmt-feat-esphome.json`](compatibility/mgmt-feat-esphome.json). Implementation starts only after those contracts and the dependency decisions are accepted.

## License

Original work is licensed under the [GNU General Public License v3.0 only](LICENSE). Imported or generated protocol material must satisfy [provenance](docs/provenance.md) and [third-party notice](THIRD_PARTY_NOTICES.md) requirements before merge.
