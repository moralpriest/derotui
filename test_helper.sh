#!/bin/bash
pkill -9 -f "derotui daemon-helper" 2>/dev/null
sleep 1
rm -f "$HOME/.derotui/daemon-helper.sock" 2>/dev/null

./derotui daemon-helper > /tmp/helper_stdout.log 2> /tmp/helper_stderr.log &
HPID=$!
echo "Started helper PID: $HPID"

sleep 2

echo '{"action":"start","settings":{"mode":"embedded","network":"mainnet","data_dir":"/home/priest/.derotui/mainnet","rpc_bind":"127.0.0.1:10102","p2p_bind":"0.0.0.0:10102","getwork_bind":"0.0.0.0:10100","fastsync":true}}' | nc -U "$HOME/.derotui/daemon-helper.sock" > /tmp/helper_response.json 2> /tmp/helper_nc.err

echo "nc exit: $?"
sleep 1

kill $HPID 2>/dev/null
wait $HPID 2>/dev/null

echo "=== STDOUT ==="
cat /tmp/helper_stdout.log 2>/dev/null || echo "(empty)"
echo "=== STDERR ==="
cat /tmp/helper_stderr.log 2>/dev/null || echo "(empty)"
echo "=== RESPONSE ==="
cat /tmp/helper_response.json 2>/dev/null || echo "(empty)"
echo "=== NC ERROR ==="
cat /tmp/helper_nc.err 2>/dev/null || echo "(empty)"
