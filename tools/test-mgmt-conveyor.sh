#!/usr/bin/env bash
set -euo pipefail

readonly expected_mcl_sha="f558027c06a0034f71ce56292fbfe6370b7200a26e2d13a3c4ef655622f8b834"

# Two bricks: the rotation starts with a red one and follows it with a blue one,
# so a single run exercises both classifications and therefore both light
# outcomes.
readonly bricks=2

if [[ "${1:-}" == "--inside" ]]; then
	shift
	readonly mgmt_root="$1"
	readonly mgmt_binary="$2"
	readonly simulator_binary="$3"
	readonly evidence_dir="$4"

	ip link set lo up
	ip link set lo multicast on
	ip route add 224.0.0.0/4 dev lo
	"${simulator_binary}" --listen 127.0.0.1:6053 --mdns-host esphome-conveyor.local \
		--cycles "${bricks}" >"${evidence_dir}/simulator.log" 2>&1 &
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

	# MGMT exits on its own: once the last brick is off the belt the device goes
	# quiet, so the converger settles and --converged-exit ends the run.
	(
		cd "${mgmt_root}"
		timeout --signal=TERM --kill-after=5s 120s "${mgmt_binary}" run \
			--tmp-prefix --converger-timeout=3 --converged-exit \
			lang examples/lang/esphome-conveyor.mcl
	) >"${evidence_dir}/mgmt.log" 2>&1

	fail() {
		echo "$1" >&2
		echo "--- mgmt.log" >&2
		cat "${evidence_dir}/mgmt.log" >&2
		echo "--- simulator.log" >&2
		cat "${evidence_dir}/simulator.log" >&2
		exit 1
	}
	want_mgmt() {
		grep -Fq "$1" "${evidence_dir}/mgmt.log" || fail "MGMT did not report: $1"
	}
	want_device() {
		grep -Fq "$1" "${evidence_dir}/simulator.log" || fail "the simulator did not receive: $1"
	}

	# The device boots with no brick en route and no colour reading, and MGMT
	# reads both. A ratio of exactly zero is the pre-subscription value and must
	# not be classified, which is why the idle light shows no colour.
	want_mgmt "print[conveyor]: Msg: en_route=false red_ratio=-1 red=false other=false"

	# A brick settles on the entry, so the device asks for a run and MGMT starts
	# the belt and blinks the light.
	want_mgmt "print[conveyor]: Msg: en_route=true red_ratio=-1 red=false other=false"
	want_mgmt "esphome:fan[Conveyor Motor]: turning fan on at speed 100 in the forward direction"
	want_mgmt "esphome:light[Status Light]: turning light on at brightness 0.35 with colour white and effect Traveling Blink"

	# The brick arrives, so the belt stops and one averaged reading arrives.
	want_mgmt "esphome:fan[Conveyor Motor]: turning fan off at speed 100 in the forward direction"
	want_mgmt "device log [info]: Exit red ratio 48.0% from 5 samples"
	want_mgmt "print[conveyor]: Msg: en_route=false red_ratio=48 red=true other=false"
	want_mgmt "esphome:light[Status Light]: turning light on at brightness 0.35 with colour red and effect "

	# The second brick is not red, so it is honestly reported as other.
	want_mgmt "device log [info]: Exit red ratio 29.0% from 5 samples"
	want_mgmt "print[conveyor]: Msg: en_route=false red_ratio=29 red=false other=true"
	want_mgmt "esphome:light[Status Light]: turning light on at brightness 0.35 with colour white and effect Rainbow"

	want_mgmt "converged for 3 seconds, exiting!"

	# The device received the decisions, not just MGMT's intent to make them.
	want_device "received fan command: state=true speed=100 direction=forward"
	want_device 'received light command: state=true brightness=0.35 rgb=#ffffff effect="Traveling Blink"'
	want_device 'received light command: state=true brightness=0.35 rgb=#ff0000 effect="None"'
	want_device 'received light command: state=true brightness=0.35 rgb=#ffffff effect="Rainbow"'

	# Returning to idle is a regression guard, not a formality. An if expression
	# nested inside the branch of another one makes MGMT drop the update that
	# switches between them, and the update this catches is the one that clears
	# a running animation. See https://github.com/purpleidea/mgmt/issues/966.
	last_effect="$(grep -F "received light command:" "${evidence_dir}/simulator.log" | tail -n 1)"
	if [[ "${last_effect}" != 'received light command: state=true brightness=0.35 rgb=#ffffff effect="None"' ]]; then
		fail "the light did not return to idle white with no effect: ${last_effect}"
	fi

	# The belt must be left stopped, whatever else happened.
	last_fan="$(grep -F "received fan command:" "${evidence_dir}/simulator.log" | tail -n 1)"
	if [[ "${last_fan}" != "received fan command: state=false speed=100 direction=forward" ]]; then
		fail "the belt was not left stopped: ${last_fan}"
	fi
	if grep -Fq "could not stop the fan on cleanup" "${evidence_dir}/mgmt.log"; then
		fail "MGMT reported a failed fan cleanup"
	fi

	# The firmware only overrides MGMT when nobody is left to decide. Neither
	# override may happen in a healthy run.
	if grep -Fq "firmware backstop" "${evidence_dir}/simulator.log"; then
		fail "the firmware jam backstop fired during a healthy run"
	fi

	echo "MGMT securely converged the reviewed conveyor MCL against the loopback simulator"
	exit 0
fi

if [[ "$#" -ne 2 ]]; then
	echo "usage: $0 MGMT_ROOT MGMT_BINARY" >&2
	exit 2
fi

readonly repo_root="$(git rev-parse --show-toplevel)"
readonly mgmt_root="$(cd "$1" && pwd)"
readonly mgmt_binary="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
readonly mcl_path="${mgmt_root}/examples/lang/esphome-conveyor.mcl"

for command in go ip sha256sum timeout unshare; do
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

(
	cd "${repo_root}"
	go build -o "${simulator_binary}" ./cmd/conveyor-sim-server
)
# The user namespace only exists so an unprivileged contributor may create
# the network namespace; root (for example in CI) creates it directly, since
# an unmapped-uid workspace would be untraversable behind --map-root-user.
unshare_flags=(--net --fork)
if [[ "$(id -u)" != "0" ]]; then
	unshare_flags=(--user --map-root-user --net --fork)
fi
unshare "${unshare_flags[@]}" \
	"$0" --inside "${mgmt_root}" "${mgmt_binary}" "${simulator_binary}" "${evidence_dir}"
