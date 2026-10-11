package tickermemev4

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KyberNetwork/kyberswap-dex-lib/pkg/source/pool"
)

// Compare against a brute-force oracle where gross output skips values. This
// catches binary searches that incorrectly assume net output never decreases.
func TestReverseAgainstExhaustiveOracle(t *testing.T) {
	for _, rates := range [][2]uint16{{0, 0}, {100, 100}, {100, 133}, {1000, 1000}, {1, 1999}} {
		for _, step := range []int64{1, 2, 7} {
			h := &Hook{Extra: terms(rates[0], rates[1])}
			grossQuote := func(param pool.CalcAmountOutParams) (*pool.CalcAmountOutResult, error) {
				return &pool.CalcAmountOutResult{TokenAmountOut: &pool.TokenAmount{Token: "out", Amount: new(big.Int).Mul(param.TokenAmountIn.Amount, big.NewInt(step))}}, nil
			}
			quote := func(param pool.CalcAmountOutParams) (*pool.CalcAmountOutResult, error) {
				result, _ := grossQuote(param)
				g := result.TokenAmountOut.Amount.Int64()
				result.TokenAmountOut.Amount.SetInt64(g - g*int64(rates[0])/10000 - g*int64(rates[1])/10000)
				result.Gas = 123
				result.SwapInfo = g
				return result, nil
			}
			for desired := int64(1); desired <= 350; desired++ {
				expected := int64(1)
				for {
					g := expected * step
					if g-g*int64(rates[0])/10000-g*int64(rates[1])/10000 >= desired {
						break
					}
					expected++
				}
				result, err := h.ReverseExactInput(pool.CalcAmountInParams{TokenIn: "in", TokenAmountOut: pool.TokenAmount{Token: "out", Amount: big.NewInt(desired)}}, quote, grossQuote)
				require.NoError(t, err, "rates=%v step=%d desired=%d", rates, step, desired)
				require.Equal(t, expected, result.TokenAmountIn.Amount.Int64())
				require.EqualValues(t, 123, result.Gas)
				require.Equal(t, expected*step, result.SwapInfo)
			}
		}
	}
}

func TestV4ReverseReplay(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, index := range []int{0, 1} {
			p := newPool(t, terms(100, 133), native)
			tokens := p.Info.Tokens
			for _, desired := range []int64{1, 99, 100, 197, 9999, 1000000} {
				param := pool.CalcAmountInParams{TokenIn: tokens[index], TokenAmountOut: pool.TokenAmount{Token: tokens[1-index], Amount: big.NewInt(desired)}}
				reverse, err := p.CalcAmountIn(param)
				require.NoError(t, err)
				replay, err := p.CalcAmountOut(pool.CalcAmountOutParams{TokenAmountIn: *reverse.TokenAmountIn, TokenOut: param.TokenAmountOut.Token})
				require.NoError(t, err)
				require.GreaterOrEqual(t, replay.TokenAmountOut.Amount.Cmp(param.TokenAmountOut.Amount), 0)
				require.Equal(t, replay.SwapInfo, reverse.SwapInfo)
				require.Equal(t, replay.Gas, reverse.Gas)
				if reverse.TokenAmountIn.Amount.Cmp(big.NewInt(1)) > 0 {
					less := new(big.Int).Sub(reverse.TokenAmountIn.Amount, big.NewInt(1))
					previous, err := p.CalcAmountOut(pool.CalcAmountOutParams{TokenAmountIn: pool.TokenAmount{Token: param.TokenIn, Amount: less}, TokenOut: param.TokenAmountOut.Token})
					if err == nil {
						require.Less(t, previous.TokenAmountOut.Amount.Cmp(param.TokenAmountOut.Amount), 0)
					}
				}
			}
		}
	}
}
