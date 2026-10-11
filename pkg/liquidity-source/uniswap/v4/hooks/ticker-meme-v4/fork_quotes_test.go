package tickermemev4

import (
	"math/big"
	"os"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/KyberNetwork/kyberswap-dex-lib/pkg/source/pool"
)

func TestDeployedHookForkQuotes(t *testing.T) {
	data, err := os.ReadFile("testdata/fork-quotes.json")
	require.NoError(t, err)
	var cases []struct {
		Name          string `json:"name"`
		Native        bool   `json:"native"`
		HookFeeBps    uint16 `json:"hookFeeBps"`
		CreatorTaxBps uint16 `json:"creatorTaxBps"`
		Trades        []struct {
			ZeroForOne         bool   `json:"zeroForOne"`
			Input              string `json:"input"`
			GrossOutput        string `json:"grossOutput"`
			NetOutput          string `json:"netOutput"`
			BeforeSqrtPriceX96 string `json:"beforeSqrtPriceX96"`
			BeforeTick         int    `json:"beforeTick"`
			AfterSqrtPriceX96  string `json:"afterSqrtPriceX96"`
			AfterTick          int    `json:"afterTick"`
		} `json:"trades"`
	}
	require.NoError(t, json.Unmarshal(data, &cases))
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			p := newPool(t, terms(scenario.HookFeeBps, scenario.CreatorTaxBps), scenario.Native)
			for _, trade := range scenario.Trades {
				require.Equal(t, trade.BeforeSqrtPriceX96, p.V3Pool.SqrtRatioX96.String())
				require.Equal(t, trade.BeforeTick, p.V3Pool.TickCurrent)
				index := 1
				if trade.ZeroForOne {
					index = 0
				}
				input, ok := new(big.Int).SetString(trade.Input, 10)
				require.True(t, ok)
				param := pool.CalcAmountOutParams{TokenAmountIn: pool.TokenAmount{Token: p.Info.Tokens[index], Amount: input}, TokenOut: p.Info.Tokens[1-index]}
				result, err := p.CalcAmountOut(param)
				require.NoError(t, err)
				require.Equal(t, trade.NetOutput, result.TokenAmountOut.Amount.String(), "net output differs from deployed hook execution")
				reverse, err := p.CalcAmountIn(pool.CalcAmountInParams{TokenIn: param.TokenAmountIn.Token, TokenAmountOut: *result.TokenAmountOut})
				require.NoError(t, err)
				require.LessOrEqual(t, reverse.TokenAmountIn.Amount.Cmp(input), 0)
				replay, err := p.CalcAmountOut(pool.CalcAmountOutParams{TokenAmountIn: *reverse.TokenAmountIn, TokenOut: param.TokenOut})
				require.NoError(t, err)
				require.GreaterOrEqual(t, replay.TokenAmountOut.Amount.Cmp(result.TokenAmountOut.Amount), 0)
				p.UpdateBalance(pool.UpdateBalanceParams{TokenAmountIn: param.TokenAmountIn, TokenAmountOut: *result.TokenAmountOut, SwapInfo: result.SwapInfo})
				require.Equal(t, trade.AfterSqrtPriceX96, p.V3Pool.SqrtRatioX96.String())
				require.Equal(t, trade.AfterTick, p.V3Pool.TickCurrent)
			}
		})
	}
}
