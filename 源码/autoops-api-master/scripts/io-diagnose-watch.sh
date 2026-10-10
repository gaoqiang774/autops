#!/usr/bin/env bash
set -Eeuo pipefail

# Diagnose whether a host freeze is caused by disk IO pressure.
# Recommended usage:
#   nohup bash scripts/io-diagnose-watch.sh > /tmp/perf-log/io-diagnose.nohup 2>&1 &
#
# Useful env overrides:
#   INTERVAL=30
#   LOG_DIR=/tmp/perf-log
#   VM_URL=http://127.0.0.1:8428
#   VM_DATA_DIR=/opt/victoriametrics/data

LOG_DIR="${LOG_DIR:-/tmp/perf-log}"
STATE_DIR="${STATE_DIR:-$LOG_DIR/io-diagnose-state}"
LOG_FILE="${LOG_FILE:-$LOG_DIR/io-diagnose-watch.log}"
INTERVAL="${INTERVAL:-30}"
SAMPLE_SECONDS="${SAMPLE_SECONDS:-1}"
SAMPLE_COUNT="${SAMPLE_COUNT:-3}"
TOP_N="${TOP_N:-20}"
VM_URL="${VM_URL:-http://127.0.0.1:8428}"
VM_DATA_DIR="${VM_DATA_DIR:-/opt/victoriametrics/data}"
BACKEND_MATCH="${BACKEND_MATCH:-aiops-backend}"
MAX_LOG_MB="${MAX_LOG_MB:-1024}"
DOCKER_TAIL_LINES="${DOCKER_TAIL_LINES:-120}"

IOWAIT_WARN="${IOWAIT_WARN:-10}"
IOWAIT_CRIT="${IOWAIT_CRIT:-25}"
BLOCKED_WARN="${BLOCKED_WARN:-1}"
DSTATE_WARN="${DSTATE_WARN:-1}"
DSTATE_ALERT_WCHAN_REGEX="${DSTATE_ALERT_WCHAN_REGEX:-io|blk|xfs|ext4|jbd|nfs|fuse|btrfs|writeback|buffer|sync|wait_on|submit_bio|dm_|nvme|scsi|ata|page}"
PSI_IO_WARN="${PSI_IO_WARN:-10}"
IO_UTIL_WARN="${IO_UTIL_WARN:-85}"
IO_AWAIT_WARN_MS="${IO_AWAIT_WARN_MS:-100}"
IO_AQ_WARN="${IO_AQ_WARN:-2}"
PROC_WRITE_WARN_BYTES="${PROC_WRITE_WARN_BYTES:-104857600}"
PROC_READ_WARN_BYTES="${PROC_READ_WARN_BYTES:-104857600}"
VM_PENDING_ROWS_WARN="${VM_PENDING_ROWS_WARN:-100000}"
VM_ACTIVE_MERGES_WARN="${VM_ACTIVE_MERGES_WARN:-2}"
VM_RSS_WARN_BYTES="${VM_RSS_WARN_BYTES:-21474836480}"
VM_FREE_DISK_WARN_BYTES="${VM_FREE_DISK_WARN_BYTES:-10737418240}"
RUN_ONCE=0

usage() {
	cat <<'EOF'
Usage:
  bash scripts/io-diagnose-watch.sh [--once] [--interval SECONDS] [--log FILE] [--vm-url URL]

Examples:
  bash scripts/io-diagnose-watch.sh --once
  nohup bash scripts/io-diagnose-watch.sh > /tmp/perf-log/io-diagnose.nohup 2>&1 &

Env:
  LOG_DIR=/tmp/perf-log
  INTERVAL=30
  VM_URL=http://127.0.0.1:8428
  VM_DATA_DIR=/opt/victoriametrics/data
  IO_UTIL_WARN=85
  IO_AWAIT_WARN_MS=100
  IOWAIT_WARN=10
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--once)
			RUN_ONCE=1
			;;
		--interval)
			INTERVAL="${2:?missing value for --interval}"
			shift
			;;
		--log)
			LOG_FILE="${2:?missing value for --log}"
			shift
			;;
		--vm-url)
			VM_URL="${2:?missing value for --vm-url}"
			shift
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			usage >&2
			exit 2
			;;
	esac
	shift
done

mkdir -p "$LOG_DIR" "$STATE_DIR"
ALERTS_FILE="$STATE_DIR/alerts.current"

cmd_exists() {
	command -v "$1" >/dev/null 2>&1
}

ge() {
	awk -v a="$1" -v b="$2" 'BEGIN { exit !((a + 0) >= (b + 0)) }'
}

log() {
	printf '%s %s\n' "$(date '+%F %T %Z')" "$*" | tee -a "$LOG_FILE"
}

section() {
	{
		printf '\n'
		printf '===== %s | %s =====\n' "$1" "$(date '+%F %T %Z')"
	} >> "$LOG_FILE"
}

alert() {
	local level="$1"
	shift
	local msg="ALERT [$level] $*"
	log "$msg"
	printf '%s\n' "$msg" >> "$ALERTS_FILE"
}

warn() {
	log "WARN $*"
}

run_block() {
	local title="$1"
	shift
	section "$title"
	if ! "$@" >> "$LOG_FILE" 2>&1; then
		warn "command failed: $*"
	fi
}

rotate_log_if_needed() {
	local max_bytes=$((MAX_LOG_MB * 1024 * 1024))
	if [[ -f "$LOG_FILE" ]]; then
		local size
		size="$(wc -c < "$LOG_FILE" 2>/dev/null || echo 0)"
		if [[ "$size" =~ ^[0-9]+$ ]] && (( size > max_bytes )); then
			local rotated="${LOG_FILE}.$(date '+%Y%m%d%H%M%S')"
			mv "$LOG_FILE" "$rotated"
			log "rotated previous log to $rotated"
		fi
	fi
}

collect_header() {
	section "sample header"
	{
		echo "host=$(hostname 2>/dev/null || echo unknown)"
		echo "kernel=$(uname -a 2>/dev/null || true)"
		echo "uptime=$(uptime 2>/dev/null || true)"
		echo "interval=$INTERVAL sample=${SAMPLE_SECONDS}s x $SAMPLE_COUNT"
		echo "vm_url=$VM_URL"
		echo "log_file=$LOG_FILE"
	} >> "$LOG_FILE"
}

collect_vmstat() {
	if ! cmd_exists vmstat; then
		warn "vmstat not found"
		return
	fi

	section "vmstat ${SAMPLE_SECONDS}s x $SAMPLE_COUNT"
	local out last r b swpd free buff cache si so bi bo interrupts cs us sy idle wa st
	out="$(vmstat "$SAMPLE_SECONDS" "$SAMPLE_COUNT" 2>&1 || true)"
	printf '%s\n' "$out" >> "$LOG_FILE"
	last="$(printf '%s\n' "$out" | awk '/^[[:space:]]*[0-9]+[[:space:]]+[0-9]+/ { line=$0 } END { print line }')"
	if [[ -z "$last" ]]; then
		return
	fi

	read -r r b swpd free buff cache si so bi bo interrupts cs us sy idle wa st <<< "$last"
	b="${b:-0}"
	wa="${wa:-0}"
	if ge "$b" "$BLOCKED_WARN"; then
		alert "WARN" "vmstat blocked processes b=$b. IO wait or uninterruptible sleep is likely."
	fi
	if ge "$wa" "$IOWAIT_CRIT"; then
		alert "CRIT" "vmstat iowait wa=${wa}% >= ${IOWAIT_CRIT}%"
	elif ge "$wa" "$IOWAIT_WARN"; then
		alert "WARN" "vmstat iowait wa=${wa}% >= ${IOWAIT_WARN}%"
	fi
}

collect_iostat() {
	if ! cmd_exists iostat; then
		alert "WARN" "iostat not found. Install sysstat to see disk await/util."
		return
	fi

	section "iostat -x ${SAMPLE_SECONDS} ${SAMPLE_COUNT}"
	local out alerts
	out="$(iostat -x "$SAMPLE_SECONDS" "$SAMPLE_COUNT" 2>&1 || true)"
	printf '%s\n' "$out" >> "$LOG_FILE"

	alerts="$STATE_DIR/iostat.alerts"
	printf '%s\n' "$out" | awk \
		-v util_warn="$IO_UTIL_WARN" \
		-v await_warn="$IO_AWAIT_WARN_MS" \
		-v aq_warn="$IO_AQ_WARN" '
		$1 == "Device" {
			report++
			delete idx
			for (i = 1; i <= NF; i++) idx[$i] = i
			next
		}
		NF == 0 || $1 == "Linux" || $1 == "avg-cpu:" { next }
		report <= 1 { next }
		idx["%util"] {
			device = $1
			util = $(idx["%util"]) + 0
			await = idx["await"] ? $(idx["await"]) + 0 : 0
			rawait = idx["r_await"] ? $(idx["r_await"]) + 0 : 0
			wawait = idx["w_await"] ? $(idx["w_await"]) + 0 : 0
			aqu = idx["aqu-sz"] ? $(idx["aqu-sz"]) + 0 : (idx["avgqu-sz"] ? $(idx["avgqu-sz"]) + 0 : 0)
			if (util >= util_warn || await >= await_warn || rawait >= await_warn || wawait >= await_warn || aqu >= aq_warn) {
				printf("device=%s util=%.2f%% await=%.2fms r_await=%.2fms w_await=%.2fms aqu-sz=%.2f\n", device, util, await, rawait, wawait, aqu)
			}
		}
	' > "$alerts"

	if [[ -s "$alerts" ]]; then
		while IFS= read -r line; do
			alert "WARN" "disk io pressure from iostat: $line"
		done < "$alerts"
	fi
}

collect_pressure() {
	section "linux pressure stall information"
	if [[ ! -r /proc/pressure/io ]]; then
		echo "/proc/pressure/io is not available" >> "$LOG_FILE"
		return
	fi

	cat /proc/pressure/io >> "$LOG_FILE"
	local some_avg10 full_avg10
	some_avg10="$(awk '/^some/ { for (i=1; i<=NF; i++) if ($i ~ /^avg10=/) { split($i, a, "="); print a[2] } }' /proc/pressure/io)"
	full_avg10="$(awk '/^full/ { for (i=1; i<=NF; i++) if ($i ~ /^avg10=/) { split($i, a, "="); print a[2] } }' /proc/pressure/io)"
	some_avg10="${some_avg10:-0}"
	full_avg10="${full_avg10:-0}"
	if ge "$some_avg10" "$PSI_IO_WARN"; then
		alert "WARN" "IO PSI some avg10=${some_avg10}% >= ${PSI_IO_WARN}%"
	fi
	if ge "$full_avg10" "$PSI_IO_WARN"; then
		alert "CRIT" "IO PSI full avg10=${full_avg10}% >= ${PSI_IO_WARN}%"
	fi
}

collect_dstate_processes() {
	section "D-state processes"
	local out actionable_count
	out="$(ps -eo pid,ppid,stat,wchan:32,comm,args 2>/dev/null | awk 'NR == 1 || $3 ~ /D/ { print }')"
	printf '%s\n' "$out" >> "$LOG_FILE"

	actionable_count="$(
		printf '%s\n' "$out" | awk -v re="$DSTATE_ALERT_WCHAN_REGEX" '
			NR > 1 {
				ppid=$2
				wchan=$4
				comm=$5
				if (ppid != 2 || wchan ~ re || comm !~ /^kworker/) c++
			}
			END { print c + 0 }
		'
	)"

	if ge "$actionable_count" "$DSTATE_WARN"; then
		alert "WARN" "found $actionable_count actionable D-state process(es). This is a strong IO/block-device symptom."
		printf '%s\n' "$out" | awk -v re="$DSTATE_ALERT_WCHAN_REGEX" '
			NR > 1 {
				ppid=$2
				wchan=$4
				comm=$5
				if (ppid != 2 || wchan ~ re || comm !~ /^kworker/) print $1
			}
		' | while read -r pid; do
			collect_process_diagnostics "$pid" "d-state"
		done
	elif [[ "$actionable_count" == "0" ]]; then
		echo "D-state entries are kernel worker/RCU-like waits only; recorded but not alerted." >> "$LOG_FILE"
	fi
}

collect_process_diagnostics() {
	local pid="$1"
	local reason="${2:-process}"
	[[ -n "$pid" && -d "/proc/$pid" ]] || return

	section "process diagnostics pid=$pid reason=$reason"
	{
		echo "ps:"
		ps -p "$pid" -o pid,ppid,stat,wchan:32,comm,pcpu,pmem,rss,etimes,args 2>/dev/null || true
		echo
		echo "cwd/exe:"
		printf 'cwd='
		readlink -f "/proc/$pid/cwd" 2>/dev/null || true
		printf 'exe='
		readlink -f "/proc/$pid/exe" 2>/dev/null || true
		echo
		echo "cmdline:"
		tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true
		echo
		echo
		echo "io:"
		cat "/proc/$pid/io" 2>/dev/null || true
		echo
		echo "limits:"
		cat "/proc/$pid/limits" 2>/dev/null || true
		echo
		echo "open files from /proc/$pid/fd:"
		ls -l "/proc/$pid/fd" 2>/dev/null | awk 'NR <= 120 { print }' || true
		echo
		echo "kernel stack:"
		cat "/proc/$pid/stack" 2>/dev/null || true
		echo
	} >> "$LOG_FILE"

	if cmd_exists lsof; then
		section "lsof pid=$pid reason=$reason"
		lsof -p "$pid" 2>/dev/null | awk 'NR <= 160 { print }' >> "$LOG_FILE" || true
	fi

	collect_container_diagnostics "$pid" "$reason"
}

extract_container_id() {
	grep -Eo '[0-9a-f]{64}' 2>/dev/null | head -1 || true
}

pid_container_id() {
	local pid="$1"
	local current="$pid"
	local parent=""
	local depth=0

	while [[ -n "$current" && -d "/proc/$current" && "$depth" -lt 10 ]]; do
		{
			cat "/proc/$current/cgroup" 2>/dev/null || true
			tr '\0' ' ' < "/proc/$current/cmdline" 2>/dev/null || true
			echo
			ps -p "$current" -o args= 2>/dev/null || true
		} | extract_container_id

		parent="$(awk '/^PPid:/ { print $2 }' "/proc/$current/status" 2>/dev/null || true)"
		[[ -n "$parent" && "$parent" != "0" && "$parent" != "$current" ]] || break
		current="$parent"
		depth=$((depth + 1))
	done
}

safe_file_size() {
	local path="$1"
	[[ -n "$path" && -e "$path" ]] || return
	ls -lh "$path" 2>/dev/null || true
	du -sh "$path" 2>/dev/null || true
}

collect_container_diagnostics() {
	local pid="$1"
	local reason="${2:-process}"
	local cid
	cid="$(pid_container_id "$pid" | head -1 || true)"
	[[ -n "$cid" ]] || return

	section "container diagnostics pid=$pid reason=$reason"
	{
		echo "container_id=$cid"
		echo "reason=$reason"
		echo
	} >> "$LOG_FILE"

	if ! cmd_exists docker; then
		echo "docker command not found; container id was inferred from /proc only" >> "$LOG_FILE"
		return
	fi

	{
		echo "docker inspect summary:"
		docker inspect --format 'id={{.Id}} name={{.Name}} image={{.Config.Image}} pid={{.State.Pid}} status={{.State.Status}} log_driver={{.HostConfig.LogConfig.Type}} log_path={{.LogPath}} upper={{.GraphDriver.Data.UpperDir}} merged={{.GraphDriver.Data.MergedDir}}' "$cid" 2>/dev/null || true
		echo
	} >> "$LOG_FILE"

	local log_path upper_dir merged_dir
	log_path="$(docker inspect --format '{{.LogPath}}' "$cid" 2>/dev/null || true)"
	upper_dir="$(docker inspect --format '{{.GraphDriver.Data.UpperDir}}' "$cid" 2>/dev/null || true)"
	merged_dir="$(docker inspect --format '{{.GraphDriver.Data.MergedDir}}' "$cid" 2>/dev/null || true)"

	if [[ -n "$log_path" && "$log_path" != "<no value>" ]]; then
		section "container log file cid=${cid:0:12}"
		{
			echo "log_path=$log_path"
			safe_file_size "$log_path"
			echo
			echo "tail -n $DOCKER_TAIL_LINES log_path:"
			tail -n "$DOCKER_TAIL_LINES" "$log_path" 2>/dev/null || true
		} >> "$LOG_FILE"
	fi

	if [[ -n "$upper_dir" && "$upper_dir" != "<no value>" && -d "$upper_dir" ]]; then
		section "container writable-layer recent files cid=${cid:0:12}"
		{
			echo "upper_dir=$upper_dir"
			du -sh "$upper_dir" 2>/dev/null || true
			find "$upper_dir" -xdev -type f -mmin -20 -printf '%TY-%Tm-%Td %TH:%TM:%TS %s %p\n' 2>/dev/null \
				| sort -k3 -n \
				| tail -80 || true
		} >> "$LOG_FILE"
	fi

	if [[ -n "$merged_dir" && "$merged_dir" != "<no value>" && -d "$merged_dir/app" ]]; then
		section "container /app recent large files cid=${cid:0:12}"
		find "$merged_dir/app" -xdev -type f -mmin -20 -size +5M -printf '%TY-%Tm-%Td %TH:%TM:%TS %s %p\n' 2>/dev/null \
			| sort -k3 -n \
			| tail -80 >> "$LOG_FILE" || true
	fi
}

snapshot_proc_io() {
	local io pid read_bytes write_bytes cancelled_write_bytes key value content
	for io in /proc/[0-9]*/io; do
		[[ -r "$io" ]] || continue
		pid="${io#/proc/}"
		pid="${pid%/io}"
		content="$(cat "$io" 2>/dev/null || true)"
		[[ -n "$content" ]] || continue
		read_bytes=0
		write_bytes=0
		cancelled_write_bytes=0
		while read -r key value; do
			case "$key" in
				read_bytes:) read_bytes="$value" ;;
				write_bytes:) write_bytes="$value" ;;
				cancelled_write_bytes:) cancelled_write_bytes="$value" ;;
			esac
		done <<< "$content"
		printf '%s %s %s %s\n' "$pid" "$read_bytes" "$write_bytes" "$cancelled_write_bytes"
	done
}

collect_proc_io_delta() {
	section "top process IO delta from /proc"
	local prev="$STATE_DIR/proc_io.prev"
	local curr="$STATE_DIR/proc_io.curr"
	local delta="$STATE_DIR/proc_io.delta"

	snapshot_proc_io > "$curr"
	if [[ ! -s "$prev" ]]; then
		echo "first sample: no process IO delta yet" >> "$LOG_FILE"
		cp "$curr" "$prev"
		return
	fi

	awk '
		NR == FNR { prev_r[$1] = $2; prev_w[$1] = $3; next }
		{
			dr = $2 - prev_r[$1]
			dw = $3 - prev_w[$1]
			if (dr < 0) dr = 0
			if (dw < 0) dw = 0
			if (dr > 0 || dw > 0) print $1, dr, dw
		}
	' "$prev" "$curr" | sort -k3,3nr -k2,2nr | awk -v limit="$TOP_N" 'NR <= limit { print }' > "$delta"

	printf '%-8s %-14s %-14s %s\n' "PID" "READ_BYTES" "WRITE_BYTES" "PROCESS" >> "$LOG_FILE"
	while read -r pid read_delta write_delta; do
		[[ -n "${pid:-}" ]] || continue
		local proc_line
		proc_line="$(ps -p "$pid" -o pid=,stat=,comm=,pcpu=,pmem=,rss=,args= 2>/dev/null || true)"
		printf '%-8s %-14s %-14s %s\n' "$pid" "$read_delta" "$write_delta" "$proc_line" >> "$LOG_FILE"
		if ge "$write_delta" "$PROC_WRITE_WARN_BYTES"; then
			alert "WARN" "process pid=$pid wrote ${write_delta} bytes during sample window: $proc_line"
			collect_process_diagnostics "$pid" "high-write-${write_delta}-bytes"
		fi
		if ge "$read_delta" "$PROC_READ_WARN_BYTES"; then
			alert "WARN" "process pid=$pid read ${read_delta} bytes during sample window: $proc_line"
			collect_process_diagnostics "$pid" "high-read-${read_delta}-bytes"
		fi
	done < "$delta"

	cp "$curr" "$prev"
}

collect_pidstat() {
	if ! cmd_exists pidstat; then
		warn "pidstat not found. Install sysstat for per-process IO samples."
		return
	fi
	run_block "pidstat -dru -h ${SAMPLE_SECONDS} ${SAMPLE_COUNT}" pidstat -dru -h "$SAMPLE_SECONDS" "$SAMPLE_COUNT"
}

collect_top_processes() {
	section "top processes by CPU"
	ps -eo pid,ppid,stat,comm,pcpu,pmem,rss,etimes,args --sort=-pcpu 2>/dev/null \
		| awk -v limit="$((TOP_N + 1))" 'NR <= limit { print }' >> "$LOG_FILE" || warn "failed to collect top CPU processes"

	section "top processes by RSS"
	ps -eo pid,ppid,stat,comm,pcpu,pmem,rss,etimes,args --sort=-rss 2>/dev/null \
		| awk -v limit="$((TOP_N + 1))" 'NR <= limit { print }' >> "$LOG_FILE" || warn "failed to collect top RSS processes"

	section "matched key processes"
	ps -eo pid,ppid,stat,comm,pcpu,pmem,rss,etimes,args 2>/dev/null \
		| awk -v pat="$BACKEND_MATCH|victoria-metric|victoria-metrics|mysqld|mysql|dockerd|containerd" 'NR == 1 || $0 ~ pat { print }' \
		>> "$LOG_FILE"
}

collect_disk_space() {
	run_block "df -h" df -h
	run_block "df -i" df -i
	if [[ -d "$VM_DATA_DIR" ]]; then
		run_block "victoriametrics data directory size" du -sh "$VM_DATA_DIR"
	fi
	if [[ -r /proc/sys/fs/file-nr ]]; then
		section "file descriptors"
		cat /proc/sys/fs/file-nr >> "$LOG_FILE"
	fi
}

collect_network() {
	if cmd_exists ss; then
		run_block "ss -s" ss -s
		section "top listening sockets"
		ss -ntlp 2>/dev/null | awk 'NR <= 80 { print }' >> "$LOG_FILE" || true
	else
		warn "ss not found"
	fi
}

metric_sum() {
	local metric_file="$1"
	local pattern="$2"
	awk -v pat="$pattern" '$0 !~ /^#/ && $1 ~ pat { sum += $NF } END { print sum + 0 }' "$metric_file"
}

collect_victoriametrics() {
	section "VictoriaMetrics health"
	local base="${VM_URL%/}"
	local metrics_file="$STATE_DIR/vm.metrics"
	local tsdb_file="$STATE_DIR/vm.tsdb.json"

	if ! cmd_exists curl; then
		warn "curl not found; cannot query VictoriaMetrics"
		return
	fi

	if curl -fsS --max-time 5 "$base/health" >> "$LOG_FILE" 2>&1; then
		echo >> "$LOG_FILE"
	else
		alert "CRIT" "VictoriaMetrics health endpoint failed: $base/health"
	fi

	section "VictoriaMetrics selected /metrics"
	if curl -fsS --max-time 10 "$base/metrics" > "$metrics_file" 2>> "$LOG_FILE"; then
		grep -E '^(vm_active|vm_rows|vm_pending|vm_merge|vm_storage|vm_cache|vm_http_request_errors|vm_concurrent|vm_slow|vm_data_size|vm_free_disk|vm_too_big|process_resident_memory_bytes|go_memstats_alloc_bytes|go_goroutines)' "$metrics_file" \
			| awk 'NR <= 300 { print }' >> "$LOG_FILE" || true

		local pending_rows active_merges vm_rss free_disk
		pending_rows="$(metric_sum "$metrics_file" '^vm_pending.*rows')"
		active_merges="$(metric_sum "$metrics_file" '^vm_active.*merge')"
		vm_rss="$(metric_sum "$metrics_file" '^process_resident_memory_bytes$')"
		free_disk="$(metric_sum "$metrics_file" '^vm_free_disk_space_bytes')"

		if ge "$pending_rows" "$VM_PENDING_ROWS_WARN"; then
			alert "WARN" "VictoriaMetrics pending rows=$pending_rows >= $VM_PENDING_ROWS_WARN"
		fi
		if ge "$active_merges" "$VM_ACTIVE_MERGES_WARN"; then
			alert "WARN" "VictoriaMetrics active merges=$active_merges >= $VM_ACTIVE_MERGES_WARN"
		fi
		if ge "$vm_rss" "$VM_RSS_WARN_BYTES"; then
			alert "WARN" "VictoriaMetrics RSS bytes=$vm_rss >= $VM_RSS_WARN_BYTES"
		fi
		if [[ "$free_disk" != "0" ]] && ge "$VM_FREE_DISK_WARN_BYTES" "$free_disk"; then
			alert "CRIT" "VictoriaMetrics free disk bytes=$free_disk <= $VM_FREE_DISK_WARN_BYTES"
		fi
	else
		alert "WARN" "failed to fetch VictoriaMetrics metrics: $base/metrics"
	fi

	section "VictoriaMetrics tsdb cardinality topN"
	if curl -fsS --max-time 15 "$base/api/v1/status/tsdb?topN=20" > "$tsdb_file" 2>> "$LOG_FILE"; then
		head -c 12000 "$tsdb_file" >> "$LOG_FILE"
		echo >> "$LOG_FILE"
	else
		warn "failed to fetch VictoriaMetrics TSDB status"
	fi
}

collect_kernel_messages() {
	if cmd_exists journalctl; then
		section "recent kernel IO/OOM/hung messages"
		journalctl -k --since "10 minutes ago" --no-pager -n 200 2>/dev/null \
			| grep -Ei 'oom|out of memory|hung|blocked for more|blocked task|i/o error|buffer i/o|reset|nvme|sda|vda|xfs|ext4|filesystem|read-only|ata[0-9]' \
			>> "$LOG_FILE" || true
	fi

	if cmd_exists dmesg; then
		section "dmesg IO/OOM/hung tail"
		dmesg -T 2>/dev/null \
			| tail -n 300 \
			| grep -Ei 'oom|out of memory|hung|blocked for more|blocked task|i/o error|buffer i/o|reset|nvme|sda|vda|xfs|ext4|filesystem|read-only|ata[0-9]' \
			>> "$LOG_FILE" || true
	fi
}

collect_one_sample() {
	: > "$ALERTS_FILE"
	rotate_log_if_needed

	section "BEGIN SAMPLE"
	collect_header
	collect_pressure
	collect_vmstat
	collect_iostat
	collect_dstate_processes
	collect_proc_io_delta
	collect_pidstat
	collect_top_processes
	collect_disk_space
	collect_network
	collect_victoriametrics
	collect_kernel_messages

	section "ALERT SUMMARY"
	if [[ -s "$ALERTS_FILE" ]]; then
		cat "$ALERTS_FILE" >> "$LOG_FILE"
	else
		echo "No IO alerts detected in this sample." >> "$LOG_FILE"
	fi
	section "END SAMPLE"
}

main() {
	log "io diagnose watcher started. log=$LOG_FILE interval=${INTERVAL}s vm_url=$VM_URL"
	if [[ "$(id -u 2>/dev/null || echo 1)" != "0" ]]; then
		warn "not running as root; some /proc, dmesg, pidstat, and process details may be hidden"
	fi

	if (( RUN_ONCE == 1 )); then
		collect_one_sample
		log "one-shot sample finished"
		return
	fi

	while true; do
		local start elapsed sleep_for
		start="$(date +%s)"
		collect_one_sample
		elapsed=$(( $(date +%s) - start ))
		sleep_for=$(( INTERVAL - elapsed ))
		if (( sleep_for < 1 )); then
			sleep_for=1
		fi
		sleep "$sleep_for"
	done
}

main
