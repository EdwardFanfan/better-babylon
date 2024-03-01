package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	bbn "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

// FinalityProviders returns a paginated list of all Babylon maintained finality providers
func (k Keeper) FinalityProviders(c context.Context, req *types.QueryFinalityProvidersRequest) (*types.QueryFinalityProvidersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	store := k.finalityProviderStore(ctx)
	currBlockHeight := uint64(ctx.BlockHeight())

	var finalityProvidersResp []*types.FinalityProviderResponse
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		var finalityProvider types.FinalityProvider
		if err := finalityProvider.Unmarshal(value); err != nil {
			return err
		}

		votingPower := k.GetVotingPower(ctx, key, currBlockHeight)
		resp := types.NewFinalityProviderResponse(&finalityProvider, currBlockHeight, votingPower)
		finalityProvidersResp = append(finalityProvidersResp, resp)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryFinalityProvidersResponse{FinalityProviders: finalityProvidersResp, Pagination: pageRes}, nil
}

// FinalityProvider returns the finality provider with the specified finality provider BTC PK
func (k Keeper) FinalityProvider(c context.Context, req *types.QueryFinalityProviderRequest) (*types.QueryFinalityProviderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if len(req.FpBtcPkHex) == 0 {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "finality provider BTC public key cannot be empty")
	}

	fpPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, err
	}

	ctx := sdk.UnwrapSDKContext(c)
	fp, err := k.GetFinalityProvider(ctx, fpPK.MustMarshal())
	if err != nil {
		return nil, err
	}

	return &types.QueryFinalityProviderResponse{FinalityProvider: fp}, nil
}

// BTCDelegations returns all BTC delegations under a given status
func (k Keeper) BTCDelegations(ctx context.Context, req *types.QueryBTCDelegationsRequest) (*types.QueryBTCDelegationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	covenantQuorum := k.GetParams(ctx).CovenantQuorum

	// get current BTC height
	btcTipHeight := k.btclcKeeper.GetTipInfo(ctx).Height
	// get value of w
	wValue := k.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout

	store := k.btcDelegationStore(ctx)
	var btcDels []*types.BTCDelegation
	pageRes, err := query.FilteredPaginate(store, req.Pagination, func(_ []byte, value []byte, accumulate bool) (bool, error) {
		var btcDel types.BTCDelegation
		k.cdc.MustUnmarshal(value, &btcDel)

		// hit if the queried status is ANY or matches the BTC delegation status
		if req.Status == types.BTCDelegationStatus_ANY || btcDel.GetStatus(btcTipHeight, wValue, covenantQuorum) == req.Status {
			if accumulate {
				btcDels = append(btcDels, &btcDel)
			}
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryBTCDelegationsResponse{
		BtcDelegations: btcDels,
		Pagination:     pageRes,
	}, nil
}

// FinalityProviderPowerAtHeight returns the voting power of the specified finality provider
// at the provided Babylon height
func (k Keeper) FinalityProviderPowerAtHeight(ctx context.Context, req *types.QueryFinalityProviderPowerAtHeightRequest) (*types.QueryFinalityProviderPowerAtHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	power := k.GetVotingPower(sdkCtx, fpBTCPK.MustMarshal(), req.Height)

	return &types.QueryFinalityProviderPowerAtHeightResponse{VotingPower: power}, nil
}

// FinalityProviderCurrentPower returns the voting power of the specified finality provider
// at the current height
func (k Keeper) FinalityProviderCurrentPower(ctx context.Context, req *types.QueryFinalityProviderCurrentPowerRequest) (*types.QueryFinalityProviderCurrentPowerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	height, power := k.GetCurrentVotingPower(ctx, *fpBTCPK)

	return &types.QueryFinalityProviderCurrentPowerResponse{Height: height, VotingPower: power}, nil
}

// ActiveFinalityProvidersAtHeight returns the active finality providers at the provided height
func (k Keeper) ActiveFinalityProvidersAtHeight(ctx context.Context, req *types.QueryActiveFinalityProvidersAtHeightRequest) (*types.QueryActiveFinalityProvidersAtHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.votingPowerStore(sdkCtx, req.Height)

	var finalityProvidersWithMeta []*types.FinalityProviderWithMeta
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		finalityProvider, err := k.GetFinalityProvider(sdkCtx, key)
		if err != nil {
			return err
		}

		votingPower := k.GetVotingPower(sdkCtx, key, req.Height)
		if votingPower > 0 {
			finalityProviderWithMeta := types.FinalityProviderWithMeta{
				BtcPk:                finalityProvider.BtcPk,
				Height:               req.Height,
				VotingPower:          votingPower,
				SlashedBabylonHeight: finalityProvider.SlashedBabylonHeight,
				SlashedBtcHeight:     finalityProvider.SlashedBtcHeight,
			}
			finalityProvidersWithMeta = append(finalityProvidersWithMeta, &finalityProviderWithMeta)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryActiveFinalityProvidersAtHeightResponse{FinalityProviders: finalityProvidersWithMeta, Pagination: pageRes}, nil
}

// ActivatedHeight returns the Babylon height in which the BTC Staking protocol was enabled
// TODO: Requires investigation on whether we can enable the BTC staking protocol at genesis
func (k Keeper) ActivatedHeight(ctx context.Context, req *types.QueryActivatedHeightRequest) (*types.QueryActivatedHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	activatedHeight, err := k.GetBTCStakingActivatedHeight(sdkCtx)
	if err != nil {
		return nil, err
	}
	return &types.QueryActivatedHeightResponse{Height: activatedHeight}, nil
}

// FinalityProviderDelegations returns all the delegations of the provided finality provider filtered by the provided status.
func (k Keeper) FinalityProviderDelegations(ctx context.Context, req *types.QueryFinalityProviderDelegationsRequest) (*types.QueryFinalityProviderDelegationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if len(req.FpBtcPkHex) == 0 {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest, "finality provider BTC public key cannot be empty")
	}

	fpPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	btcDelStore := k.btcDelegatorStore(sdkCtx, fpPK)

	currentWValue := k.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	btcHeight := k.btclcKeeper.GetTipInfo(ctx).Height
	covenantQuorum := k.GetParams(ctx).CovenantQuorum

	btcDels := []*types.BTCDelegatorDelegationsResponse{}
	pageRes, err := query.Paginate(btcDelStore, req.Pagination, func(key, value []byte) error {
		delBTCPK, err := bbn.NewBIP340PubKey(key)
		if err != nil {
			return err
		}

		curBTCDels, err := k.getBTCDelegatorDelegations(sdkCtx, fpPK, delBTCPK)
		if err != nil {
			return err
		}

		btcDelsResp := make([]*types.BTCDelegationResponse, len(curBTCDels.Dels))
		for i, btcDel := range curBTCDels.Dels {
			status := btcDel.GetStatus(
				btcHeight,
				currentWValue,
				covenantQuorum,
			)
			btcDelsResp[i] = types.NewBTCDelegationResponse(btcDel, status)
		}

		btcDels = append(btcDels, &types.BTCDelegatorDelegationsResponse{
			Dels: btcDelsResp,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryFinalityProviderDelegationsResponse{BtcDelegatorDelegations: btcDels, Pagination: pageRes}, nil
}

// BTCDelegation returns existing btc delegation by staking tx hash
func (k Keeper) BTCDelegation(ctx context.Context, req *types.QueryBTCDelegationRequest) (*types.QueryBTCDelegationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	// decode staking tx hash
	stakingTxHash, err := chainhash.NewHashFromStr(req.StakingTxHashHex)
	if err != nil {
		return nil, err
	}

	// find BTC delegation
	btcDel := k.getBTCDelegation(ctx, *stakingTxHash)
	if btcDel == nil {
		return nil, types.ErrBTCDelegationNotFound
	}

	currentWValue := k.btccKeeper.GetParams(ctx).CheckpointFinalizationTimeout
	status := btcDel.GetStatus(
		k.btclcKeeper.GetTipInfo(ctx).Height,
		currentWValue,
		k.GetParams(ctx).CovenantQuorum,
	)

	return &types.QueryBTCDelegationResponse{
		BtcDelegation: types.NewBTCDelegationResponse(btcDel, status),
	}, nil
}
