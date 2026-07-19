#!/usr/bin/env bash
# Run MGMT's unchanged examples/lang/esphome-blink.mcl against the blink-device
# simulator inside a private network namespace. Without a cycle count the demo
# streams the endless cooperative blink loop until Ctrl-C. With a cycle count
# it stops after that many observed blink cycles and verifies the evidence.
set -euo pipefail

readonly expected_blink_sha="359cedc5b3fd1e6793a0705fc4d7c7f844f5d3dc825a372fdf0c6769ef30c187"
readonly off_command_line="received switch command: key=212 state=false"
# One complete blink cycle is proven by MGMT receiving the firmware's relight
# log; counting received off commands would overcount MGMT's startup burst.
readonly relight_line="device log [info]: LED is still off; turning it on"

if [[ "${1:-}" == "--inside" ]]; then
	shift
	readonly mgmt_root="$1"
	readonly mgmt_binary="$2"
	readonly simulator_binary="$3"
	readonly work_dir="$4"
	readonly cycles="$5"

	ip link set lo up
	ip link set lo multicast on
	ip route add 224.0.0.0/4 dev lo

	simulator_log="${work_dir}/blink-simulator.log"
	mgmt_log="${work_dir}/blink-mgmt.log"
	: >"${simulator_log}"
	: >"${mgmt_log}"
	simulator_pid=""
	tail_pid=""
	mgmt_pid=""
	cleanup() {
		for pid in "${mgmt_pid}" "${tail_pid}" "${simulator_pid}"; do
			[[ -n "${pid}" ]] && kill "${pid}" 2>/dev/null || true
		done
		[[ -n "${mgmt_pid}" ]] && wait "${mgmt_pid}" 2>/dev/null || true
		[[ -n "${simulator_pid}" ]] && wait "${simulator_pid}" 2>/dev/null || true
	}
	trap cleanup EXIT

	"${simulator_binary}" --scenario blink-device --listen 127.0.0.1:6053 \
		--mdns-host esphome-blink.local >"${simulator_log}" 2>&1 &
	simulator_pid=$!
	for _ in $(seq 1 100); do
		if grep -Fq "secure blink-device simulator ready" "${simulator_log}"; then
			break
		fi
		if ! kill -0 "${simulator_pid}" 2>/dev/null; then
			cat "${simulator_log}" >&2
			exit 1
		fi
		sleep 0.05
	done
	grep -Fq "secure blink-device simulator ready" "${simulator_log}"

	if [[ "${cycles}" == "0" ]]; then
		echo "The simulated device is up. MGMT now runs the unchanged"
		echo "examples/lang/esphome-blink.mcl and cannot tell this is not real"
		echo "hardware: it turns the LED off whenever the device reports it on,"
		echo "and the simulated firmware turns it back on three seconds later."
		echo "Watch for '[simulator] ${off_command_line}',"
		echo "'[simulator] simulated firmware relit the LED', and mgmt's own"
		echo "'on-board led is on' lines. Press Ctrl-C to stop."
		echo
		tail -n +1 -F "${simulator_log}" 2>/dev/null > >(sed -u 's/^/[simulator] /') &
		tail_pid=$!
		(
			cd "${mgmt_root}"
			exec "${mgmt_binary}" run --tmp-prefix lang examples/lang/esphome-blink.mcl
		) &
		mgmt_pid=$!
		# Ctrl-C reaches every process in the foreground group. Absorb it here
		# and keep waiting so MGMT finishes its own graceful shutdown first.
		interrupted=0
		trap 'interrupted=1' INT
		mgmt_status=0
		while kill -0 "${mgmt_pid}" 2>/dev/null; do
			wait "${mgmt_pid}" 2>/dev/null && mgmt_status=0 || mgmt_status=$?
		done
		mgmt_pid=""
		# Without an interrupt the loop never ends by itself, so reaching this
		# point means MGMT failed; do not report a crash as a clean stop.
		if [[ "${interrupted}" != "1" ]]; then
			echo "MGMT exited unexpectedly with status ${mgmt_status}" >&2
			tail -n 40 "${simulator_log}" >&2
			exit 1
		fi
		echo "blink demo stopped"
		exit 0
	fi

	(
		cd "${mgmt_root}"
		# exec keeps the tracked PID on timeout itself, which forwards the
		# stop signal to MGMT instead of orphaning it in the namespace.
		exec timeout --signal=TERM --kill-after=5s 300s \
			"${mgmt_binary}" run --tmp-prefix lang examples/lang/esphome-blink.mcl
	) >"${mgmt_log}" 2>&1 &
	mgmt_pid=$!

	deadline=$((SECONDS + 60 + cycles * 10))
	while true; do
		observed="$(grep -Fc "${relight_line}" "${mgmt_log}" || true)"
		if [[ "${observed}" -ge "${cycles}" ]]; then
			break
		fi
		if ! kill -0 "${mgmt_pid}" 2>/dev/null || [[ "${SECONDS}" -ge "${deadline}" ]]; then
			echo "observed only ${observed} of ${cycles} blink cycles" >&2
			tail -n 40 "${simulator_log}" "${mgmt_log}" >&2
			exit 1
		fi
		sleep 0.2
	done

	# A healthy loop never ends by itself: MGMT must still be running when the
	# target count is reached, otherwise a crash after the last relight would
	# be reported as success.
	if ! kill -0 "${mgmt_pid}" 2>/dev/null; then
		echo "MGMT exited on its own after ${observed} observed blink cycles" >&2
		tail -n 40 "${simulator_log}" "${mgmt_log}" >&2
		exit 1
	fi
	kill -TERM "${mgmt_pid}" 2>/dev/null || true
	wait "${mgmt_pid}" 2>/dev/null || true
	mgmt_pid=""

	fail() {
		echo "missing expected demo evidence: $1" >&2
		tail -n 40 "${simulator_log}" "${mgmt_log}" >&2
		exit 1
	}
	for needle in \
		"print[led state]: Msg: on-board led is on: true" \
		"print[led state]: Msg: on-board led is on: false" \
		"esphome:switch[Onboard LED]: turning off" \
		"device log [info]: blink simulator ready" \
		"device log [info]: LED turned off; turning it back on in three seconds" \
		"device log [info]: LED is still off; turning it on"; do
		grep -Fq "${needle}" "${mgmt_log}" || fail "${needle}"
	done

	echo "MGMT blinked the unchanged blink MCL for ${cycles} cycles against the loopback simulator"
	exit 0
fi

if [[ "$#" -lt 2 || "$#" -gt 3 ]]; then
	echo "usage: $0 MGMT_ROOT MGMT_BINARY [BLINK_CYCLES]" >&2
	exit 2
fi

readonly repo_root="$(git rev-parse --show-toplevel)"
readonly mgmt_root="$(cd "$1" && pwd)"
readonly mgmt_binary="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
readonly cycles="${3:-0}"
readonly blink_path="${mgmt_root}/examples/lang/esphome-blink.mcl"

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
if ! [[ "${cycles}" =~ ^[0-9]+$ ]]; then
	echo "BLINK_CYCLES must be a whole number: ${cycles}" >&2
	exit 2
fi
if [[ "$(sha256sum "${blink_path}" | cut -d' ' -f1)" != "${expected_blink_sha}" ]]; then
	echo "esphome-blink.mcl differs from the reviewed compatibility contract" >&2
	exit 1
fi

work_dir="$(mktemp -d)"
cleanup() { rm -rf "${work_dir}"; }
trap cleanup EXIT
simulator_binary="${work_dir}/mgmt-compat-sim-server"

(
	cd "${repo_root}"
	go build -o "${simulator_binary}" ./cmd/mgmt-compat-sim-server
)
# The user namespace only exists so an unprivileged contributor may create
# the network namespace; root (for example in CI) creates it directly, since
# an unmapped-uid workspace would be untraversable behind --map-root-user.
unshare_flags=(--net --fork)
if [[ "$(id -u)" != "0" ]]; then
	unshare_flags=(--user --map-root-user --net --fork)
fi
unshare "${unshare_flags[@]}" \
	"$0" --inside "${mgmt_root}" "${mgmt_binary}" "${simulator_binary}" "${work_dir}" "${cycles}"
