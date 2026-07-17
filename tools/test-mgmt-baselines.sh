#!/usr/bin/env bash
set -euo pipefail

readonly expected_esphome0_sha="8a5ba295eb0a649af89592c0f42899d0078c642fa521c73a7224e00304daa7df"
readonly expected_blink_sha="cc57833875290b60e7e7f1004b93d00fb17249ad2f31267ac20ff91c1052c7ad"

if [[ "${1:-}" == "--inside" ]]; then
	shift
	readonly mgmt_root="$1"
	readonly mgmt_binary="$2"
	readonly simulator_binary="$3"
	readonly evidence_dir="$4"
	readonly parent_netns="$5"

	ip link set lo up
	ip link set lo multicast on
	ip route add 224.0.0.0/4 dev lo
	ip address add 192.168.1.50/32 dev lo

	run_case() {
		local name="$1"
		local mcl="$2"
		shift 2
		local simulator_log="${evidence_dir}/${name}-simulator.log"
		local mgmt_log="${evidence_dir}/${name}-mgmt.log"

		"${simulator_binary}" "$@" >"${simulator_log}" 2>&1 &
		local simulator_pid=$!
		for _ in $(seq 1 100); do
			if grep -Fq "secure ${name} simulator ready" "${simulator_log}"; then
				break
			fi
			if ! kill -0 "${simulator_pid}" 2>/dev/null; then
				cat "${simulator_log}" >&2
				exit 1
			fi
			sleep 0.05
		done
		grep -Fq "secure ${name} simulator ready" "${simulator_log}"

		(
			cd "${mgmt_root}"
			timeout --signal=TERM --kill-after=5s 40s "${mgmt_binary}" run \
				--tmp-prefix --converger-timeout=3 --converged-exit \
				lang "examples/lang/${mcl}"
		) >"${mgmt_log}" 2>&1

		kill "${simulator_pid}" 2>/dev/null || true
		wait "${simulator_pid}" 2>/dev/null || true
	}

	run_case "basic-io" "esphome0.mcl" \
		--scenario basic-io --listen 127.0.0.1:6054 \
		--forward-listen 192.168.1.50:6053 --parent-netns "${parent_netns}"
	grep -Fq "print[connected]: Msg: garage is connected: true" "${evidence_dir}/basic-io-mgmt.log"
	grep -Fq "esphome:switch[led_1]: turning off" "${evidence_dir}/basic-io-mgmt.log"
	grep -Fq "esphome:number[motor_speed]: setting value to 0" "${evidence_dir}/basic-io-mgmt.log"
	grep -Fq "converged for 3 seconds, exiting!" "${evidence_dir}/basic-io-mgmt.log"
	grep -Fq "received switch command: key=202 state=false" "${evidence_dir}/basic-io-simulator.log"
	grep -Fq "received number command: key=203 value=0" "${evidence_dir}/basic-io-simulator.log"

	run_case "blink" "esphome-blink.mcl" \
		--scenario blink --listen 127.0.0.1:6053 --mdns-host esphome-blink.local
	grep -Fq "print[led state]: Msg: on-board led is on: true" "${evidence_dir}/blink-mgmt.log"
	grep -Fq "esphome:switch[Onboard LED]: turning off" "${evidence_dir}/blink-mgmt.log"
	grep -Fq "device log [info]: blink simulator ready" "${evidence_dir}/blink-mgmt.log"
	grep -Fq "converged for 3 seconds, exiting!" "${evidence_dir}/blink-mgmt.log"
	grep -Fq "received switch command: key=212 state=false" "${evidence_dir}/blink-simulator.log"

	echo "MGMT securely converged both unchanged baseline MCL examples against dedicated simulators"
	exit 0
fi

if [[ "$#" -ne 2 ]]; then
	echo "usage: $0 MGMT_ROOT MGMT_BINARY" >&2
	exit 2
fi

readonly repo_root="$(git rev-parse --show-toplevel)"
readonly mgmt_root="$(cd "$1" && pwd)"
readonly mgmt_binary="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
readonly esphome0_path="${mgmt_root}/examples/lang/esphome0.mcl"
readonly blink_path="${mgmt_root}/examples/lang/esphome-blink.mcl"

for command in go ip readlink sha256sum timeout unshare; do
	if ! command -v "${command}" >/dev/null 2>&1; then
		echo "required command is missing: ${command}" >&2
		exit 1
	fi
done
if [[ ! -x "${mgmt_binary}" ]]; then
	echo "MGMT binary is not executable: ${mgmt_binary}" >&2
	exit 1
fi
if [[ "$(sha256sum "${esphome0_path}" | cut -d' ' -f1)" != "${expected_esphome0_sha}" ]]; then
	echo "esphome0.mcl differs from the reviewed compatibility contract" >&2
	exit 1
fi
if [[ "$(sha256sum "${blink_path}" | cut -d' ' -f1)" != "${expected_blink_sha}" ]]; then
	echo "esphome-blink.mcl differs from the reviewed compatibility contract" >&2
	exit 1
fi

evidence_dir="$(mktemp -d)"
cleanup() { rm -rf "${evidence_dir}"; }
trap cleanup EXIT
simulator_binary="${evidence_dir}/mgmt-compat-sim-server"
parent_netns="$(readlink /proc/self/ns/net)"

(
	cd "${repo_root}"
	go build -o "${simulator_binary}" ./cmd/mgmt-compat-sim-server
)
unshare --user --map-root-user --net --fork \
	"$0" --inside "${mgmt_root}" "${mgmt_binary}" "${simulator_binary}" "${evidence_dir}" "${parent_netns}"
