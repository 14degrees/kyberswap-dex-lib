#!/bin/sh
set -eu
: "${TICKER_SOURCE:?Set TICKER_SOURCE to the Ticker contracts checkout at b0d654362ed9fc7599cda0ee127cfb8c791938f8}"
: "${TICKER_MEME_RPC:?Set TICKER_MEME_RPC to a Monad RPC with block 112361144 available}"
task_source=$(cd "$TICKER_SOURCE" && pwd)
task_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
forge test --root "$task_root" \
  --remappings "forge-std/=$task_source/contracts-ticker/lib/forge-std/src/" \
  --remappings "v4-core/=$task_source/contracts-ticker/lib/v4-core/src/" \
  --remappings "ticker/=$task_source/contracts-ticker/src/" \
  -j 1 -vv
