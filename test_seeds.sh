#!/bin/bash
# Test each seed node for DERO handshake

SEEDS=(
  "82.65.143.182:11011"
  "85.214.253.170:58686"
  "38.180.116.63:11011"
  "213.171.208.37:18089"
  "190.194.227.11:11011"
  "51.222.86.51:11011"
  "209.145.59.4:50404"
  "116.111.112.188:11011"
)

for seed in "${SEEDS[@]}"; do
  echo "=== Testing $seed ==="
  
  pkill -9 -f "derotui daemon-helper" 2>/dev/null
  sleep 1
  rm -rf "$HOME/.derotui/mainnet" "$HOME/.derotui/daemon-helper.sock" 2>/dev/null
  
  ./derotui daemon-helper > /tmp/seed_test_${seed//[:\/]/_}.log 2>&1 &
  sleep 2
  
  echo '{"action":"start","settings":{"mode":"embedded","network":"mainnet","data_dir":"/home/priest/.derotui","rpc_bind":"127.0.0.1:10102","p2p_bind":"0.0.0.0:10102","getwork_bind":"0.0.0.0:10100","fastsync":true,"add_exclusive_node":["'$seed'"]}}' | nc -U "$HOME/.derotui/daemon-helper.sock" > /dev/null
  
  sleep 10
  
  echo '{"action":"status"}' | nc -U "$HOME/.derotui/daemon-helper.sock" | jq -r '.info.incoming_peers // 0, .info.known_peers // 0, .info.height // 0'
  
  # Check for any successful connection
  grep -i "connected\|handshake.*ok\|peer list" /tmp/seed_test_${seed//[:\/]/_}.log | tail -3
  
  pkill -9 -f "derotui daemon-helper" 2>/dev/null
  echo ""
done

echo "=== SUMMARY ==="
for seed in "${SEEDS[@]}"; do
  log="/tmp/seed_test_${seed//[:\/]/_}.log"
  if grep -q "cannot handshake" "$log" && ! grep -q "connected to" "$log"; then
    echo "$seed: FAILED (handshake timeout)"
  elif grep -q "connected to" "$log" || grep -q "peer list" "$log"; then
    echo "$seed: SUCCESS"
  else
    echo "$seed: UNKNOWN"
  fi
done
