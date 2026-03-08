#!/bin/bash
# Echo 压测脚本：支持小字段/大字段、少连接/多连接等场景
# 用法: ./run_echo_bench.sh [all|small|large|multi|1k|1k1k]
#   all   - 跑全部场景（默认）
#   small - 仅小字段 20 连接
#   large - 仅大字段 20 连接
#   multi - 仅小字段 100 连接
#   1k    - 仅小字段 1000 连接
#   1k1k  - 1KB  payload 1000 连接（与 gnet 同条件对比）

set -e

cd "$(dirname "$0")"

# 场景预设
# small: 23B, 20 conn, 50万 msg
# large: 1024B, 20 conn, 10万 msg
# multi: 23B, 100 conn, 50万 msg
# 1k:    23B, 1000 conn, 50万 msg
# 1k1k:  1KB, 1000 conn, 10万 msg（与 gnet 同条件对比）
SMALL_TOTAL=500000
SMALL_CLIENTS=20
SMALL_SIZE=23

LARGE_TOTAL=100000
LARGE_CLIENTS=20
LARGE_SIZE=1024

MULTI_TOTAL=500000
MULTI_CLIENTS=100
MULTI_SIZE=23

CONN1K_TOTAL=500000
CONN1K_CLIENTS=1000
CONN1K_SIZE=23

CONN1K1K_TOTAL=100000
CONN1K1K_CLIENTS=1000
CONN1K1K_SIZE=1024

MODE="${1:-all}"

get_port() {
	case "$1" in
		tcp) echo 9001 ;;
		ws)  echo 9002 ;;
		kcp) echo 9003 ;;
		*)   echo 9001 ;;
	esac
}

free_ports() {
	for proto in tcp ws kcp; do
		port=$(get_port "$proto")
		pid=$(lsof -ti :$port 2>/dev/null) || true
		if [ -n "$pid" ]; then
			echo ">>> 释放端口 $port (PID $pid)..."
			kill $pid 2>/dev/null || true
		fi
	done
	sleep 2
}

run_scenario() {
	local proto=$1
	local name=$2
	local total=$3
	local clients=$4
	local size=$5
	local extra="${6:-}"
	local port
	local addr
	local log_suffix

	port=$(get_port "$proto")
	addr="127.0.0.1:$port"
	log_suffix="${proto}_${name}"

	echo ""
	echo ">>> [$proto] $name: ${size}B x ${clients} 连接, ${total} 消息"
	go run ./examples/echobench/client -bench -addr "$addr" -p "$proto" \
		-n "$total" -c "$clients" -size "$size" $extra 2>&1 | tee "/tmp/echo_bench_${log_suffix}.log"
}

run_protocol() {
	local proto=$1
	local port
	local server_pid

	port=$(get_port "$proto")
	echo ""
	echo ">>> [$proto] 启动服务端 (端口 $port)..."
	go run ./examples/echobench/server -p "$proto" -addr ":$port" -quiet >/dev/null 2>&1 &
	server_pid=$!
	sleep 3

	case "$MODE" in
		small)
			run_scenario "$proto" "small" "$SMALL_TOTAL" "$SMALL_CLIENTS" "$SMALL_SIZE"
			;;
		large)
			run_scenario "$proto" "large" "$LARGE_TOTAL" "$LARGE_CLIENTS" "$LARGE_SIZE"
			;;
		multi)
			run_scenario "$proto" "multi" "$MULTI_TOTAL" "$MULTI_CLIENTS" "$MULTI_SIZE"
			;;
		1k)
			run_scenario "$proto" "1k" "$CONN1K_TOTAL" "$CONN1K_CLIENTS" "$CONN1K_SIZE"
			;;
		1k1k)
			run_scenario "$proto" "1k1k" "$CONN1K1K_TOTAL" "$CONN1K1K_CLIENTS" "$CONN1K1K_SIZE"
			;;
		all)
			# 1k1k 最重，先跑避免前序场景影响
			run_scenario "$proto" "1k1k" "$CONN1K1K_TOTAL" "$CONN1K1K_CLIENTS" "$CONN1K1K_SIZE"
			run_scenario "$proto" "small" "$SMALL_TOTAL" "$SMALL_CLIENTS" "$SMALL_SIZE"
			run_scenario "$proto" "large" "$LARGE_TOTAL" "$LARGE_CLIENTS" "$LARGE_SIZE"
			run_scenario "$proto" "multi" "$MULTI_TOTAL" "$MULTI_CLIENTS" "$MULTI_SIZE"
			run_scenario "$proto" "1k" "$CONN1K_TOTAL" "$CONN1K_CLIENTS" "$CONN1K_SIZE"
			;;
		*)
			echo "未知模式: $MODE，使用 all"
			run_scenario "$proto" "1k1k" "$CONN1K1K_TOTAL" "$CONN1K1K_CLIENTS" "$CONN1K1K_SIZE"
			run_scenario "$proto" "small" "$SMALL_TOTAL" "$SMALL_CLIENTS" "$SMALL_SIZE"
			run_scenario "$proto" "large" "$LARGE_TOTAL" "$LARGE_CLIENTS" "$LARGE_SIZE"
			run_scenario "$proto" "multi" "$MULTI_TOTAL" "$MULTI_CLIENTS" "$MULTI_SIZE"
			run_scenario "$proto" "1k" "$CONN1K_TOTAL" "$CONN1K_CLIENTS" "$CONN1K_SIZE"
			;;
	esac

	kill $server_pid 2>/dev/null || true
	for _ in 1 2 3 4 5; do
		kill -0 $server_pid 2>/dev/null || break
		sleep 0.5
	done
	kill -9 $server_pid 2>/dev/null || true
	wait $server_pid 2>/dev/null || true
	sleep 1
}

echo "=========================================="
echo "  zhenyi-base Echo 压测"
echo "  模式: $MODE"
echo "=========================================="

free_ports
run_protocol tcp
run_protocol ws
run_protocol kcp

echo ""
echo "=========================================="
echo "  压测完成，结果汇总："
echo "=========================================="
for proto in tcp ws kcp; do
	for name in small large multi 1k 1k1k; do
		log="/tmp/echo_bench_${proto}_${name}.log"
		if [ -f "$log" ]; then
			qps=$(grep "qps:" "$log" | tail -1 | awk '{print $2}')
			size_info=""
			case "$name" in
				small) size_info="23B/20c" ;;
				large) size_info="1KB/20c" ;;
				multi) size_info="23B/100c" ;;
				1k)    size_info="23B/1000c" ;;
				1k1k)  size_info="1KB/1000c" ;;
			esac
			echo "  [$proto] $size_info QPS: $qps msg/s"
		fi
	done
done
echo ""
