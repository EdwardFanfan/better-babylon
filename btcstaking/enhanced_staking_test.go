package btcstaking_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/btcstaking"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func generateTxFromOutputs(r *rand.Rand, info *btcstaking.EnhancedStakingInfo) (*wire.MsgTx, int, int) {
	numOutputs := 20

	stakingOutputIdx := r.Intn(numOutputs - 1)
	opReturnOutputIdx := r.Intn(numOutputs - 1)

	if stakingOutputIdx == opReturnOutputIdx {
		opReturnOutputIdx = stakingOutputIdx + 1
	}
	tx := wire.NewMsgTx(2)
	for i := 0; i < numOutputs; i++ {
		if i == stakingOutputIdx {
			tx.AddTxOut(info.StakingOutput)
		} else if i == opReturnOutputIdx {
			tx.AddTxOut(info.OpReturnOutput)
		} else {
			tx.AddTxOut(wire.NewTxOut(int64(r.Int63n(1000000000)+10000), datagen.GenRandomByteArray(r, 32)))
		}
	}

	return tx, stakingOutputIdx, opReturnOutputIdx
}

// Property: Every staking tx generated by our generator should be properly parsed by
// our parser
func FuzzGenerateAndParseValidV0StakingTransaction(f *testing.F) {
	// lot of seeds as test is pretty fast and we want to test a lot of different values
	datagen.AddRandomSeedsToFuzzer(f, 100)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// 3 - 10 covenants
		numCovenantKeys := uint32(r.Int31n(7) + 3)
		quroum := uint32(numCovenantKeys - 2)
		stakingAmount := btcutil.Amount(r.Int63n(1000000000) + 10000)
		stakingTime := uint16(r.Int31n(math.MaxUint16-1) + 1)
		magicBytes := datagen.GenRandomByteArray(r, btcstaking.MagicBytesLen)
		net := &chaincfg.MainNetParams

		sc := GenerateTestScenario(r, t, 1, numCovenantKeys, quroum, stakingAmount, stakingTime)

		outputs, err := btcstaking.BuildV0EnhancedStakingOutputs(
			magicBytes,
			sc.StakerKey.PubKey(),
			sc.FinalityProviderKeys[0].PubKey(),
			sc.CovenantPublicKeys(),
			quroum,
			stakingTime,
			stakingAmount,
			net,
		)

		require.NoError(t, err)
		require.NotNil(t, outputs)

		tx, stakingOutputIdx, opReturnOutputIdx := generateTxFromOutputs(r, outputs)

		// ParseV0StakingTx and IsPossibleV0StakingTx should be consistent and recognize
		// the same tx as a valid staking tx
		require.True(t, btcstaking.IsPossibleV0StakingTx(tx, magicBytes))

		parsedTx, err := btcstaking.ParseV0StakingTx(
			tx,
			magicBytes,
			sc.CovenantPublicKeys(),
			quroum,
			net,
		)
		require.NoError(t, err)
		require.NotNil(t, parsedTx)

		require.Equal(t, outputs.StakingOutput.PkScript, parsedTx.StakingOutput.PkScript)
		require.Equal(t, outputs.StakingOutput.Value, parsedTx.StakingOutput.Value)
		require.Equal(t, stakingOutputIdx, parsedTx.StakingOutputIdx)

		require.Equal(t, outputs.OpReturnOutput.PkScript, parsedTx.OpReturnOutput.PkScript)
		require.Equal(t, outputs.OpReturnOutput.Value, parsedTx.OpReturnOutput.Value)
		require.Equal(t, opReturnOutputIdx, parsedTx.OpReturnOutputIdx)

		require.Equal(t, magicBytes, parsedTx.OpReturnData.MagicBytes)
		require.Equal(t, uint8(0), parsedTx.OpReturnData.Version)
		require.Equal(t, stakingTime, parsedTx.OpReturnData.StakingTime)

		require.Equal(t, schnorr.SerializePubKey(sc.StakerKey.PubKey()), parsedTx.OpReturnData.StakerPublicKey.Marshall())
		require.Equal(t, schnorr.SerializePubKey(sc.FinalityProviderKeys[0].PubKey()), parsedTx.OpReturnData.FinalityProviderPublicKey.Marshall())
	})
}

// TODO Negative test cases
