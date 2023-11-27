package types

import (
	"bytes"
	"encoding/hex"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonchain/babylon/btcstaking"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/types"
)

type BTCSlashingTx []byte

func NewBTCSlashingTxFromMsgTx(msgTx *wire.MsgTx) (*BTCSlashingTx, error) {
	var buf bytes.Buffer
	err := msgTx.Serialize(&buf)
	if err != nil {
		return nil, err
	}

	tx := BTCSlashingTx(buf.Bytes())
	return &tx, nil
}

func NewBTCSlashingTxFromHex(txHex string) (*BTCSlashingTx, error) {
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}
	var tx BTCSlashingTx
	if err := tx.Unmarshal(txBytes); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (tx BTCSlashingTx) Marshal() ([]byte, error) {
	return tx, nil
}

func (tx BTCSlashingTx) MustMarshal() []byte {
	txBytes, err := tx.Marshal()
	if err != nil {
		panic(err)
	}
	return txBytes
}

func (tx BTCSlashingTx) MarshalTo(data []byte) (int, error) {
	bz, err := tx.Marshal()
	if err != nil {
		return 0, err
	}
	copy(data, bz)
	return len(data), nil
}

func (tx *BTCSlashingTx) Unmarshal(data []byte) error {
	*tx = data

	// ensure data can be decoded to a tx
	if _, err := tx.ToMsgTx(); err != nil {
		return err
	}

	return nil
}

func (tx *BTCSlashingTx) Size() int {
	return len(tx.MustMarshal())
}

func (tx *BTCSlashingTx) ToHexStr() string {
	txBytes := tx.MustMarshal()
	return hex.EncodeToString(txBytes)
}

func (tx *BTCSlashingTx) ToMsgTx() (*wire.MsgTx, error) {
	return ParseBtcTx(*tx)
}

func (tx *BTCSlashingTx) Validate(
	net *chaincfg.Params,
	slashingAddress string,
	slashingRate sdkmath.LegacyDec,
	slashingTxMinFee, stakingOutputValue int64,
) error {
	msgTx, err := tx.ToMsgTx()
	if err != nil {
		return err
	}
	decodedAddr, err := btcutil.DecodeAddress(slashingAddress, net)
	if err != nil {
		return err
	}
	return btcstaking.ValidateSlashingTx(
		msgTx,
		decodedAddr,
		slashingRate,
		slashingTxMinFee, stakingOutputValue,
	)
}

// Sign generates a signature on the slashing tx
func (tx *BTCSlashingTx) Sign(
	stakingMsgTx *wire.MsgTx,
	spendOutputIndex uint32,
	scriptPath []byte,
	sk *btcec.PrivateKey,
) (*types.BIP340Signature, error) {
	msgTx, err := tx.ToMsgTx()
	if err != nil {
		return nil, err
	}
	schnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		msgTx,
		stakingMsgTx,
		spendOutputIndex,
		scriptPath,
		sk,
	)
	if err != nil {
		return nil, err
	}
	sig := types.NewBIP340SignatureFromBTCSig(schnorrSig)
	return &sig, nil
}

// VerifySignature verifies a signature on the slashing tx signed by staker, validator or covenant
func (tx *BTCSlashingTx) VerifySignature(
	stakingPkScript []byte,
	stakingAmount int64,
	stakingScript []byte,
	pk *btcec.PublicKey,
	sig *types.BIP340Signature) error {
	msgTx, err := tx.ToMsgTx()
	if err != nil {
		return err
	}
	return btcstaking.VerifyTransactionSigWithOutputData(
		msgTx,
		stakingPkScript,
		stakingAmount,
		stakingScript,
		pk,
		*sig,
	)
}

// EncSign generates an adaptor signature on the slashing tx with validator's
// public key as encryption key
func (tx *BTCSlashingTx) EncSign(
	stakingMsgTx *wire.MsgTx,
	spendOutputIndex uint32,
	scriptPath []byte,
	sk *btcec.PrivateKey,
	encKey *asig.EncryptionKey,
) (*asig.AdaptorSignature, error) {
	msgTx, err := tx.ToMsgTx()
	if err != nil {
		return nil, err
	}
	adaptorSig, err := btcstaking.EncSignTxWithOneScriptSpendInputStrict(
		msgTx,
		stakingMsgTx,
		spendOutputIndex,
		scriptPath,
		sk,
		encKey,
	)
	if err != nil {
		return nil, err
	}

	return adaptorSig, nil
}

// EncVerifyAdaptorSignature verifies an adaptor signature on the slashing tx
// with the validator's public key as encryption key
func (tx *BTCSlashingTx) EncVerifyAdaptorSignature(
	stakingPkScript []byte,
	stakingAmount int64,
	stakingScript []byte,
	pk *btcec.PublicKey,
	encKey *asig.EncryptionKey,
	sig *asig.AdaptorSignature,
) error {
	msgTx, err := tx.ToMsgTx()
	if err != nil {
		return err
	}
	return btcstaking.EncVerifyTransactionSigWithOutputData(
		msgTx,
		stakingPkScript,
		stakingAmount,
		stakingScript,
		pk,
		encKey,
		sig,
	)
}
