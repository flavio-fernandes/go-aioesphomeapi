#!/usr/bin/env bash
set -euo pipefail

readonly expected_mcl_sha="c746b6382afaba3daaa248860402fd788ed379e889bbfde65b402efde422fc8b"

if [[ "${1:-}" == "--inside" ]]; then
	shift
	readonly mgmt_root="$1"
	readonly mgmt_binary="$2"
	readonly simulator_binary="$3"
	readonly hosts_file="$4"
	readonly evidence_dir="$5"

	ip link set lo up
	mount --bind "${hosts_file}" /etc/hosts
	"${simulator_binary}" --listen 127.0.0.1:6053 >"${evidence_dir}/simulator.log" 2>&1 &
	simulator_pid=$!
	cleanup() {
		kill "${simulator_pid}" 2>/dev/null || true
		wait "${simulator_pid}" 2>/dev/null || true
	}
	trap cleanup EXIT

	for _ in $(seq 1 100); do
		if grep -Fq "secure conveyor simulator listening" "${evidence_dir}/simulator.log"; then
			break
		fi
		if ! kill -0 "${simulator_pid}" 2>/dev/null; then
			cat "${evidence_dir}/simulator.log" >&2
			exit 1
		fi
		sleep 0.05
	done
	grep -Fq "secure conveyor simulator listening" "${evidence_dir}/simulator.log"

	(
		cd "${mgmt_root}"
		timeout --signal=TERM --kill-after=5s 30s "${mgmt_binary}" run \
			--tmp-prefix --converger-timeout=3 --converged-exit \
			lang examples/lang/esphome-conveyer.mcl
	) >"${evidence_dir}/mgmt.log" 2>&1

	grep -Fq "print[conveyor telemetry]: Msg: entry=false exit=false run=false rgb=(0, 0, 0) status=blue" "${evidence_dir}/mgmt.log"
	grep -Fq "esphome:fan[Conveyor Motor]: turning fan off at speed 35 in the forward direction" "${evidence_dir}/mgmt.log"
	grep -Fq "esphome:light[Status Light]: turning light on at brightness 0.35 with color blue" "${evidence_dir}/mgmt.log"
	grep -Fq "device log [info]: conveyor simulator ready" "${evidence_dir}/mgmt.log"
	grep -Fq "converged for 3 seconds, exiting!" "${evidence_dir}/mgmt.log"
	grep -Fq "received fan command: state=false speed=35 direction=forward" "${evidence_dir}/simulator.log"
	grep -Fq "received light command: state=true brightness=0.35 rgb=#0000ff" "${evidence_dir}/simulator.log"
	for _ in $(seq 1 20); do
		if [[ "$(grep -Fc "received fan command: state=false speed=35 direction=forward" "${evidence_dir}/simulator.log")" -ge 2 ]]; then
			break
		fi
		sleep 0.05
	done
	if [[ "$(grep -Fc "received fan command: state=false speed=35 direction=forward" "${evidence_dir}/simulator.log")" -lt 2 ]]; then
		echo "MGMT did not send the expected fan stop during cleanup" >&2
		cat "${evidence_dir}/mgmt.log" >&2
		cat "${evidence_dir}/simulator.log" >&2
		exit 1
	fi
	if grep -Fq "could not stop the fan on cleanup" "${evidence_dir}/mgmt.log"; then
		echo "MGMT reported a failed fan cleanup" >&2
		exit 1
	fi

	echo "MGMT securely converged the unchanged conveyor MCL against the loopback simulator"
	exit 0
fi

if [[ "$#" -ne 2 ]]; then
	echo "usage: $0 MGMT_ROOT MGMT_BINARY" >&2
	exit 2
fi

readonly repo_root="$(git rev-parse --show-toplevel)"
readonly mgmt_root="$(cd "$1" && pwd)"
readonly mgmt_binary="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
readonly mcl_path="${mgmt_root}/examples/lang/esphome-conveyer.mcl"

for command in go ip mount sha256sum timeout unshare; do
	if ! command -v "${command}" >/dev/null 2>&1; then
		echo "required command is missing: ${command}" >&2
		exit 1
	fi
done
if [[ ! -x "${mgmt_binary}" ]]; then
	echo "MGMT binary is not executable: ${mgmt_binary}" >&2
	exit 1
fi
if [[ ! -f "${mcl_path}" ]]; then
	echo "conveyor MCL is missing from MGMT_ROOT" >&2
	exit 1
fi
actual_mcl_sha="$(sha256sum "${mcl_path}" | cut -d' ' -f1)"
if [[ "${actual_mcl_sha}" != "${expected_mcl_sha}" ]]; then
	echo "conveyor MCL hash differs from the reviewed compatibility contract" >&2
	exit 1
fi

evidence_dir="$(mktemp -d)"
cleanup() { rm -rf "${evidence_dir}"; }
trap cleanup EXIT
simulator_binary="${evidence_dir}/conveyor-sim-server"
hosts_file="${evidence_dir}/hosts"

(
	cd "${repo_root}"
	go build -o "${simulator_binary}" ./cmd/conveyor-sim-server
)
cp /etc/hosts "${hosts_file}"
printf '127.0.0.1\tesphome-conveyer.local\n' >>"${hosts_file}"

unshare --user --map-root-user --mount --net --fork \
	"$0" --inside "${mgmt_root}" "${mgmt_binary}" "${simulator_binary}" "${hosts_file}" "${evidence_dir}"
