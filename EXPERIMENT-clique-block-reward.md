# EXPERIMENT: Clique block reward

**Status: not merged, not enabled. Do not deploy.**

This branch exists to answer one question and preserve the answer:

> Can a Clique (PoA) chain issue native tokens to the block producer as blocks
> are sealed, the way PoW does with a coinbase reward?

Short answer: **yes, mechanically, but it is not the same thing economically,
and Jouleverse mainnet must never enable it.**

---

## Why upstream Clique mints nothing

`Clique.Finalize` is a deliberate no-op on state:

```go
func (c *Clique) Finalize(...) {
    // No block rewards in PoA, so the state remains as is and uncles are dropped
    header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
    header.UncleHash = types.CalcUncleHash(nil)
}
```

There is no `accumulateRewards` anywhere in `consensus/clique/`. Every Joule so
far has come from one place: the genesis allocation.

That is why Jouleverse distributes through a contract instead. The genesis EOA
`0xa99f930222cc6490706897ce6c5611d29d42a5f9` was allocated 1e29 wei, and the
`Wrapped Joule` contract at `0x7fba9bb966189db8c4fe33b7bf67bfa24203c6ad`
(`name() = "Wrapped Joule"`, `symbol() = "WJ"`, `decimals = 18`) mints 1:1
against native J held in reserve. Distribution is `mint(address, amount)` calls
plus `withdraw(uint256)` to turn WJ back into spendable J.

---

## What this branch changes

Two files:

- `params/blockreward.go` — new. `BlockReward{Amount, Cap}` and
  `RewardFor(chainID)`.
  - chain 3666 (mainnet), 1 (Ethereum), 1337 (local dev) → zero, forever
  - every other chain ID → 1 J per block, capped at 21,000,000 J total
- `consensus/clique/clique.go` — `Finalize` now calls `accumulateReward`.

The schedule lives in code, not in the config file, for the same reason as the
transfer rules: a node must not be able to change a consensus outcome by
editing a toml.

---

## Trap 1 — `header.Coinbase` is not the block producer in Clique

Copying Ethash's `state.AddBalance(header.Coinbase, reward)` sends the reward to
the wrong place. In Clique, `header.Coinbase` carries the **address currently
being voted on**:

- `Prepare` picks a random pending proposal and puts it in `header.Coinbase`
  (and sets `Nonce` to an auth/drop vote marker)
- with no pending proposals, it is the zero address
- `verifyHeader` **requires it to be zero on every checkpoint block**
  (`number % Epoch == 0`), returning `errInvalidCheckpointBeneficiary` otherwise

So the reward would either go to whichever node is being voted in, or to
`0x000...000`, and would be burned at every 30,000th block.

The producer is recorded in exactly one place: the 65-byte signature at the tail
of `header.Extra`.

## Trap 2 — `Finalize` runs *before* `Seal` writes the signature

The obvious fix — `ecrecover(header)` inside `Finalize` — does not work for a
locally sealing node, because the header it is finalizing is still unsigned.

The first implementation did exactly that. Observed result: the chain kept
sealing normally, the log filled with

```
WARN clique: cannot recover signer for block reward number=1 err="recovery failed"
```

and the signer's balance stayed at **zero across nine blocks**. There is no
error at the RPC layer, no failed block, nothing that looks broken. The reward
simply never arrives. This is the failure mode to watch for.

The fix separates the two paths in `rewardPayee`:

- recover from the signature first — this is the path a **verifying** node takes
  for a block that arrived over the wire, and it must be identical on every node
  or the state roots diverge
- fall back to `c.signer` — the path a **sealing** node takes for its own
  not-yet-signed header

---

## Verified behaviour

Built with `go build -p 2`, single-signer Clique chains, period 2s.

| chainID | blocks sealed | signer balance | `header.Coinbase` | value transfer |
|---|---|---|---|---|
| 3888 | 15 | **11.0 J** | `0x000...000` throughout | rejected: `not allowed to transfer gas energy` |
| 3666 | 13 | **0** (no issuance) | `0x000...000` | n/a |

Reward is received but cannot be transferred — it can only be burned as gas,
which is the intended effect for a test network: signers get gas money without
the reward becoming a transferable asset.

On 3888 a `value: 0` transaction paid from reward succeeded, spending gas and
leaving a residual balance. A `value: 1 J` transfer from the same account was
still rejected by the existing transfer regulation.

---

## Why mainnet must never enable this

1. **It would be inflation of a fixed-supply chain.** Jouleverse's entire supply
   was set at genesis and is distributed through WJ. Adding a reward later mints
   tokens that nobody agreed to.
2. **It changes the state root.** Enabling it at any height is a hard fork and
   requires every signer and witness node to upgrade in lockstep.
3. **It is not a security budget.** PoW rewards compensate real cost (energy,
   hardware) and scale with difficulty. Clique has no difficulty — it is
   constant 1 or 2 — and block rate is fixed by `period`. Issuance here is a
   pure linear function of time: 1 J/block at 15s is ~5,760 J/day, ~2.1M J/year,
   regardless of who runs nodes or how much work they do. Signers are
   permissioned and identifiable, so their marginal cost is near zero. The
   reward is a minting privilege, not a payment for work.

Clique cannot express "issuance proportional to work" because there is no work
to measure. That is a property of the consensus mechanism, not a missing feature.

---

## If a new test network needs gas money

Use this branch. The pattern that works:

1. Leave `alloc` empty (or with nominal balances only). Do not pre-fund a faucet.
2. Start the chain on any chain ID other than 1, 3666 or 1337.
3. Signers accrue 1 J per block and can spend it on gas immediately.
4. Transfers stay blocked by `params.RegulationFor`, so the reward cannot be
   moved between accounts — only burned.

If rewards should ever be transferable, that is a separate decision requiring a
change to the transfer regulation, not just to this file.

---

## Cleanup when the regulation work lands

`params/blockreward.go` re-declares `JouleverseMainnetChainID`,
`EthereumMainnetChainID` and `LocalDevChainID`, because this branch was cut from
`master` which does not have `params/regulation.go`. Once the regulation branch
merges, delete the duplicated `const` block at the bottom of the file and use the
originals.
