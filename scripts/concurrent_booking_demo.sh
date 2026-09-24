#!/usr/bin/env bash
# Demonstrates the CreateBooking race guard: fires two concurrent requests
# for the exact same room/time range and shows that exactly one succeeds.
#
# Usage: ./scripts/concurrent_booking_demo.sh [base_url]
# Requires: the server running (go run main.go) and `python3` for JSON parsing.
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"

echo "== seeding office/room/users =="
OFFICE_ID=$(curl -s -X POST "$BASE_URL/offices" -d '{"location":"Blr HQ"}' -H 'Content-Type: application/json' | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
ROOM_ID=$(curl -s -X POST "$BASE_URL/rooms" -d "{\"office_id\":$OFFICE_ID,\"timezone\":\"UTC\"}" -H 'Content-Type: application/json' | python3 -c "import sys,json;print(json.load(sys.stdin)['room_id'])")
U1_ID=$(curl -s -X POST "$BASE_URL/users" -d "{\"name\":\"Racer A\",\"email\":\"racer-a-$$@example.com\"}" -H 'Content-Type: application/json' | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
U2_ID=$(curl -s -X POST "$BASE_URL/users" -d "{\"name\":\"Racer B\",\"email\":\"racer-b-$$@example.com\"}" -H 'Content-Type: application/json' | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
START=$(date -u -v+5H +%Y-%m-%dT%H:00:00Z 2>/dev/null || date -u -d "+5 hour" +%Y-%m-%dT%H:00:00Z)
echo "office=$OFFICE_ID room=$ROOM_ID host_A=$U1_ID host_B=$U2_ID start=$START"
echo

echo "== firing two concurrent CreateBooking requests for the same room/time =="
TMP=$(mktemp -d)
( curl -s -o "$TMP/A.json" -w "%{http_code}" -X POST "$BASE_URL/bookings" \
    -H 'Content-Type: application/json' \
    -d "{\"room_id\":$ROOM_ID,\"start_time\":\"$START\",\"duration_seconds\":1800,\"host_user_id\":$U1_ID}" \
    > "$TMP/A.status" ) &
( curl -s -o "$TMP/B.json" -w "%{http_code}" -X POST "$BASE_URL/bookings" \
    -H 'Content-Type: application/json' \
    -d "{\"room_id\":$ROOM_ID,\"start_time\":\"$START\",\"duration_seconds\":1800,\"host_user_id\":$U2_ID}" \
    > "$TMP/B.status" ) &
wait

echo "-- Request A (host $U1_ID): HTTP $(cat "$TMP/A.status") --"
cat "$TMP/A.json"; echo
echo "-- Request B (host $U2_ID): HTTP $(cat "$TMP/B.status") --"
cat "$TMP/B.json"; echo
echo

A_STATUS=$(cat "$TMP/A.status")
B_STATUS=$(cat "$TMP/B.status")
if { [ "$A_STATUS" = "201" ] && [ "$B_STATUS" = "409" ]; } || { [ "$A_STATUS" = "409" ] && [ "$B_STATUS" = "201" ]; }; then
  echo "RESULT: exactly one request won (201), the other was rejected (409) — race guard holds."
else
  echo "RESULT: unexpected — got A=$A_STATUS B=$B_STATUS"
fi

# Clean up whichever booking won so re-running this script stays repeatable.
WINNER_SLOT=$(python3 -c "
import json
for f in ['$TMP/A.json', '$TMP/B.json']:
    try:
        d = json.load(open(f))
        if 'slot_id' in d:
            print(d['slot_id']); break
    except Exception:
        pass
")
if [ -n "${WINNER_SLOT:-}" ]; then
  curl -s -o /dev/null -X DELETE "$BASE_URL/bookings/$WINNER_SLOT"
  echo "cleaned up booking slot_id=$WINNER_SLOT"
fi
rm -rf "$TMP"
