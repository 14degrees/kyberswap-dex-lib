# TickerMemeHook

`ticker-meme-v4` supports Ticker's classic graduated Uniswap V4 pools on Monad
(chain 143). The existing V4 lister discovers pools through PoolManager
initialization events; its tracker supplies ticks, liquidity, LP/protocol fees,
native currency flags and a snapshot block. This hook adds the launch-specific
fees through the same factories. `entity.Pool.Type` remains `uniswap-v4`; its
exchange is `ticker-meme-v4`.

Contract reference: [TickerMemeHook at b0d65436](https://github.com/Ticker-gg/ticker-contracts/blob/b0d654362ed9fc7599cda0ee127cfb8c791938f8/contracts-ticker/src/v2/hooks/TickerMemeHook.sol).
The contracts repository requires access. Uniswap's [hook delta rules](https://github.com/Uniswap/v4-core/blob/main/src/libraries/Hooks.sol)
describe how an afterSwap return adjusts the swapper's balance.

| Contract | Monad address |
| --- | --- |
| Hook proxy | [`0x223f8b1b43aeec86d2659cce36169f72f7fc2044`](https://monadvision.com/address/0x223F8B1b43AEEc86d2659cCe36169f72f7FC2044) |
| Implementation | [`0x2d96e4e89d37551ff55eb9cfead78da1278d5986`](https://monadvision.com/address/0x2D96E4e89D37551Ff55eB9cfEaD78DA1278D5986) |
| PoolManager | [`0x188d586ddcf52439676ca21a244753fa19f9ea8e`](https://monadvision.com/address/0x188d586ddcf52439676ca21a244753fa19f9ea8e) |

The proxy enables beforeInitialize, afterSwap and afterSwapReturnDelta
(`0x2044`). The implementation's runtime hash, proxy runtime hash and snapshot
block/hash are recorded in [testdata/deployment.json](testdata/deployment.json).
At block **112361144**, the proxy's `factory()` was still zero. The fork harness
wires it and creates new pools only inside its local fork. The recorded trades
are local fork executions, not production transactions.

## Pricing and tracking

PoolManager runs ordinary V4 math, including the pool's LP/protocol fee, then
calls TickerMemeHook. The hook charges
`floor(grossOutput * hookFeeBps / 10000) + floor(grossOutput * creatorTaxBps / 10000)`
for exact-input swaps. A basis point is 0.01%. With two 100 bps cuts and gross
output 199, each cut is 1 and net output is 197. At gross output 200, each cut
becomes 2 and net output falls to 196. Combining the rates before rounding is
incorrect.

Kyber's `CalcAmountIn` finds input for an eventual exact-input execution. It
therefore reverses these output cuts. The optional V4 reverse interface searches
the monotone gross output and verifies the resulting net output; it never binary
searches net output across simultaneous fee rounding boundaries. It returns the
minimum input and the exact-input quote's gas and swap state.

`Track` reads the thirteen-field `launches(poolId)` getter at the V4 snapshot
block and stores the two rates separately in HookExtra. It rechecks the ERC-1967
implementation slot and runtime hash on every refresh, respects state/code
overrides, and rejects unsupported upgrades. Quotes reject missing/malformed
state and fee terms outside the contract's limits. Unregistered pools charge
zero, matching the contract. Native MON uses V4's existing WMON representation
and native execution metadata.

Fee collection changes pending hook balances, not the pool's post-swap tick or
liquidity. Fee conversion and buyback happen during later sweeps. The V4 core
SwapInfo already supplies everything UpdateBalance needs. Hook terms are value
fields; CloneState copies them, and msgpack registers the concrete hook type.

## Verification

```sh
CI=true go test ./pkg/liquidity-source/uniswap/v4/... ./pkg/msgpack ./pkg/pooltypes
CI=true go test -race ./pkg/liquidity-source/uniswap/v4/hooks/ticker-meme-v4
TICKER_MEME_RPC=https://rpc.monad.xyz go test ./pkg/liquidity-source/uniswap/v4/hooks/ticker-meme-v4 -run TestTrackLiveDeployment -v
go generate ./pkg/msgpack/...
```

[testdata/fork-quotes.json](testdata/fork-quotes.json) contains sixteen sequential
swaps on the real Monad PoolManager and deployed hook at block 112361144: ERC-20
and native pairs, both directions, creator tax 0/100/133 bps, fee dust and zero
fees. Go tests compare outputs, sqrt prices and ticks exactly after every trade.
Reverse quotes also replay successfully; a separate exhaustive oracle tests
minimum input through rounding dips and skipped gross output values. RPC tests
cover block pinning, overrides and upgrade rejection. Serialization tests cover
forward and reverse quotes after a full simulator round trip.

To reproduce the contract executions, check out the Ticker contract reference
above in a separate directory, then run:

```sh
TICKER_SOURCE=/absolute/path/to/ticker-contracts \
TICKER_MEME_RPC=https://rpc.monad.xyz \
sh pkg/liquidity-source/uniswap/v4/hooks/ticker-meme-v4/testdata/solidity/run.sh
```

The harness uses Foundry and Solidity 0.8.30. It writes four small scenario JSON
files next to itself; combine them in ERC-20, native, rounding, zero-fees order
to reproduce the fixture. Token contracts and pool liquidity are local test
fixtures. The testdata module keeps the harness and runtime fixture out of
downstream Go module downloads.

The model adds a conservative **100,000 gas** allowance when a fee is taken.
Recorded ERC-20 fork trades show up to 91,698 extra gas versus zero-fee trades
with the same pool/trade setup. Production token/router gas still needs
calibration after the factory is wired. This change models classic MemeHook
pools; a bonding curve and other graduation venues have their own pricing.
