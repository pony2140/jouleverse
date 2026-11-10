// Copyright 2026 The Jouleverse Authors
//
// License: LGPL
//
// This file is part of an EXPERIMENTAL branch. It is not merged and not
// intended to be enabled on any live network. See the package comment below.

package params

import "math/big"

// BlockReward configures the native-token block reward of a chain.
//
// Clique (EIP-225) mints nothing: Clique.Finalize leaves the state untouched,
// so a genesis allocation is the only supply a Clique chain will ever have. A
// chain that wants issuance proportional to block production has to add it
// explicitly, because there is no uncle mechanism and no difficulty to work
// with.
//
// EXPERIMENTAL. Nothing in the default configuration reaches this code path
// for a production chain: Jouleverse mainnet, Ethereum mainnet and the local
// development chain all map to a zero reward. It exists so that a freshly
// bootstrapped test network can hand its signers gas money without shipping a
// pre-funded faucet.
//
// The values live in code for the same reason as params.RegulationFor: node
// configuration must not be able to change a consensus outcome.
type BlockReward struct {
	// Amount is paid to the block producer for every sealed block.
	// nil or zero means no reward.
	Amount *big.Int

	// Cap is the total amount of rewards this chain may ever issue, counted
	// from genesis. nil means uncapped.
	//
	// This is deliberately a function of the block number rather than a
	// counter in state, so every node agrees on the running total without
	// keeping any extra bookkeeping.
	Cap *big.Int
}

// RewardFor returns the block reward schedule of the given chain ID.
//
//   - Jouleverse mainnet (3666) issues nothing. Its entire supply was
//     allocated at genesis and distributed through a contract (Wrapped Joule).
//     Adding a reward now would silently inflate a supply that was designed to
//     be fixed, and would be a hard fork of the state root.
//   - Ethereum mainnet (1) and the local dev chain (1337) issue nothing here.
//     This binary runs neither as a production network, and the upstream test
//     fixtures assume zero issuance.
//   - Every other chain ID, including the testnet (3888) and any chain a
//     developer spins up, issues JouleverseTestnetBlockReward per block, capped
//     at JouleverseTestnetRewardCap.
//
// A nil chain ID is not a chain: it only appears in partially constructed
// configurations used by chain generation and tracing tooling. It is reported
// as unregulated so those paths keep working.
func RewardFor(chainID *big.Int) BlockReward {
	if chainID == nil {
		return BlockReward{}
	}
	if chainID.Cmp(big.NewInt(EthereumMainnetChainID)) == 0 ||
		chainID.Cmp(big.NewInt(LocalDevChainID)) == 0 ||
		chainID.Cmp(big.NewInt(JouleverseMainnetChainID)) == 0 {
		return BlockReward{}
	}
	return BlockReward{
		Amount: big.NewInt(JouleverseTestnetBlockReward),
		Cap:    new(big.Int).Set(JouleverseTestnetRewardCap),
	}
}

const (
	// JouleverseTestnetBlockReward is the per-block issuance of a chain that
	// did not opt out. 1 J per block at a 15 second period is roughly 5,760 J
	// per day (about 2.1 million J per year), which is enough to fund gas for
	// a test network without turning it into a meaningful store of value.
	JouleverseTestnetBlockReward = 1e18
)

// JouleverseTestnetRewardCap bounds the total issuance of a non-mainnet chain
// at 21,000,000 J. At 1 J per block the cap is reached after 21 million blocks,
// i.e. about ten years at a 15 second period. In practice the cap exists only
// so that "this chain cannot inflate forever" is a checkable property rather
// than a promise.
var JouleverseTestnetRewardCap = new(big.Int).Mul(big.NewInt(21_000_000), big.NewInt(1e18))

// The chain IDs below are duplicated from the regulation experiment so that
// this branch can stand alone against master. When the regulation work lands,
// delete these and use the constants from params/regulation.go.
const (
	// JouleverseMainnetChainID identifies the Jouleverse mainnet.
	JouleverseMainnetChainID = 3666

	// EthereumMainnetChainID is the chain ID of the Ethereum main network.
	EthereumMainnetChainID = 1

	// LocalDevChainID is the conventional chain ID for a local development
	// chain (Hardhat, Ganache).
	LocalDevChainID = 1337
)
