#!/bin/sh
# MCP stdio adapter: bridges one real MCP client to a running vmcp
# server.  The client spawns this script as its server command.  It
# stamps the transport identity: every message is wrapped in an
# envelope with this session's name, and the client's initialize gains
# this session's token.  The client never handles its own identity.
#
# args: SESSION TOKEN DIR
# DIR must hold control.in (FIFO the server reads) and out.log (the
# server's output).  Writers serialize on control.lock so envelope
# lines never interleave.
S="$1"; TOKEN="$2"; DIR="$3"

tail -s 0.1 -F "$DIR/out.log" 2>/dev/null \
  | jq --unbuffered -c --arg s "$S" 'select(.session == $s) | .payload' \
  | while IFS= read -r r; do
      printf '%s rx %s\n' "$(date +%s.%N)" "$r" >> "$DIR/wire-$S.log"
      printf '%s\n' "$r"
    done &
TAILPID=$!
trap 'kill $TAILPID 2>/dev/null' EXIT INT TERM

while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*)
      line=$(printf '%s' "$line" | jq -c --arg t "$TOKEN" '.params = ((.params // {}) + {token: $t})')
      ;;
  esac
  envline=$(printf '%s' "$line" | jq -c --arg s "$S" '{session: $s, payload: .}')
  printf '%s tx %s\n' "$(date +%s.%N)" "$line" >> "$DIR/wire-$S.log"
  ( flock 9; printf '%s\n' "$envline" > "$DIR/control.in" ) 9>> "$DIR/control.lock"
done
