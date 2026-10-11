package msgpack

import (
	"math/big"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/KyberNetwork/kyberswap-dex-lib/pkg/entity"
	uniswapv4 "github.com/KyberNetwork/kyberswap-dex-lib/pkg/liquidity-source/uniswap/v4"
	tickermemev4 "github.com/KyberNetwork/kyberswap-dex-lib/pkg/liquidity-source/uniswap/v4/hooks/ticker-meme-v4"
	"github.com/KyberNetwork/kyberswap-dex-lib/pkg/source/pool"
	"github.com/KyberNetwork/kyberswap-dex-lib/pkg/valueobject"
)

func TestTickerMemeV4Serialization(t *testing.T) {
	hx, err := json.Marshal(tickermemev4.Extra{ImplementationHash: tickermemev4.ImplementationCodeHash, Registered: true, HookFeeBps: 100, CreatorTaxBps: 133})
	require.NoError(t, err)
	static, err := json.Marshal(map[string]any{"0x0": [2]bool{true, false}, "fee": 2000, "tS": 60, "hooks": tickermemev4.HookAddress})
	require.NoError(t, err)
	ep := entity.Pool{Address: "0x1111111111111111111111111111111111111111111111111111111111111111", Type: uniswapv4.DexType, Exchange: tickermemev4.DexType, SwapFee: 2000, Reserves: []string{"1000000000000", "1000000000000"}, Tokens: []*entity.PoolToken{{Address: valueobject.LowerWrapped(valueobject.ChainIDMonad), Decimals: 18}, {Address: "0xf000000000000000000000000000000000000000", Decimals: 18}}, StaticExtra: string(static), BlockNumber: 112361144,
		Extra: `{"liquidity":1000000000000,"sqrtPriceX96":79228162514264337593543950336,"tickSpacing":60,"tick":0,"ticks":[{"index":-887220,"liquidityGross":1000000000000,"liquidityNet":1000000000000},{"index":887220,"liquidityGross":1000000000000,"liquidityNet":-1000000000000}],"hX":` + string(hx) + `}`,
	}
	before, err := uniswapv4.NewPoolSimulator(ep, valueobject.ChainIDMonad)
	require.NoError(t, err)
	data, err := EncodePoolSimulatorsMap(map[string]pool.IPoolSimulator{ep.Address: before})
	require.NoError(t, err)
	decoded, err := DecodePoolSimulatorsMap(data)
	require.NoError(t, err)
	after := decoded[ep.Address].(*uniswapv4.PoolSimulator)
	for _, index := range []int{0, 1} {
		param := pool.CalcAmountOutParams{TokenAmountIn: pool.TokenAmount{Token: ep.Tokens[index].Address, Amount: big.NewInt(1000000)}, TokenOut: ep.Tokens[1-index].Address}
		expected, err := before.CalcAmountOut(param)
		require.NoError(t, err)
		actual, err := after.CalcAmountOut(param)
		require.NoError(t, err)
		require.Equal(t, expected.TokenAmountOut, actual.TokenAmountOut)
		require.Equal(t, expected.SwapInfo, actual.SwapInfo)
		require.Equal(t, before.GetExchange(), after.GetExchange())
		reverseParam := pool.CalcAmountInParams{TokenIn: param.TokenAmountIn.Token, TokenAmountOut: *actual.TokenAmountOut}
		expectedIn, err := before.CalcAmountIn(reverseParam)
		require.NoError(t, err)
		actualIn, err := after.CalcAmountIn(reverseParam)
		require.NoError(t, err)
		require.Equal(t, expectedIn.TokenAmountIn, actualIn.TokenAmountIn)
	}
}
