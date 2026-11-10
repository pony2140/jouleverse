# EXPERIMENT: let each signer choose its own fee recipient

**Status: prototype, verified on a two-node chain. NOT merged, NOT activated.**

This is a different design from the two earlier branches, and the difference
matters:

| branch | who picks the recipient | applies to a running chain |
|---|---|---|
| `experiment/gas-beneficiary` | the chain, in genesis | no |
| `experiment/fee-recipient-fork` | the chain, at a fixed height | yes, hard fork |
| **`experiment/miner-fee-recipient`** | **each node, for its own blocks** | **yes, no fork** |

The requirement was the third one: a signer should be free to name any address
it likes, the way a Bitcoin miner writes an arbitrary address into its coinbase
output. No coordinated address, no vote, no genesis change.

---

## Why this is possible at all

On Bitcoin the miner's freedom exists because the coinbase transaction is
constructed by the miner and its output address is not something the consensus
rules constrain. Clique has no coinbase transaction, so the equivalent has to be
a field the header already dedicates to the producer.

`header.Coinbase` looks like the obvious candidate and is not usable:

- it carries the current **vote target** (`Prepare` picks a pending proposal)
- it must be zero when there is no pending proposal
- it is forced to zero on checkpoint blocks (`errInvalidCheckpointBeneficiary`)

So the recipient goes into the 32-byte field Clique already reserves and
currently requires to be zero: `header.MixDigest`. Twelve zero bytes of padding
followed by the 20-byte address.

Three properties make this safe:

1. **It is covered by the seal.** `CliqueRLP` includes `MixDigest`, so the value
   is part of what the signer signs. A relaying node cannot rewrite it without
   invalidating the signature.
2. **It needs no local account.** The address is only ever *published*, never
   signed with. A contract wallet works as well as an externally held key.
3. **It is per-block, so per-node.** Each node writes its own value into the
   blocks it seals, and credits whatever value it finds in blocks sealed by
   others. No agreement is needed because there is nothing to agree on.

---

## The change

8 files, ~170 lines, no genesis edit.

- `consensus/consensus.go` — new optional interface `FeeRecipientEngine`, so
  engines that lack the feature (ethash) and external implementers are unaffected
- `consensus/clique/clique.go` — `FeeRecipient()` reads the header field;
  `Prepare()` writes it; header verification accepts a non-zero field once the
  feature is active, with the padding bytes required to be zero
- `consensus/clique/feerecipient.go` — `FeeRecipientActivation` height gate
- `consensus/beacon/consensus.go` — the wrapper forwards the call to the inner
  engine. **This one is easy to miss and silently breaks everything**: the
  engine the chain exposes is the `beacon.Beacon` wrapper, so a type assertion
  straight to `*clique.Clique` fails and the feature looks implemented but never
  fires. It cost a full debugging cycle here.
- `core/evm.go` — one lookup in `NewEVMBlockContext`, the single point where the
  sealing and verifying paths converge
- `eth/backend.go` — apply `Miner.FeeRecipient` to the engine at `StartMining`
- `cmd/utils/flags.go`, `cmd/geth/main.go` — `--miner.feerecipient`

### Activation

`clique.FeeRecipientActivation` gates *participation*, not validity. Below it
nobody may use the field; above it any node may. A node that never upgrades
keeps producing blocks with the field zero, and those blocks stay valid forever.

This is why no fork is needed: nodes that ignore the feature and nodes that use
it accept each other's blocks, because the field is simply part of the block
that each signer chose to produce.

Default is 0 (active from genesis). For mainnet it should be set to a height
that gives every operator time to upgrade, so that the first block using it does
not surprise an old node — not because old nodes would reject it, but because
"old binary sees a non-zero reserved field" is a failure mode worth staging.

---

## Verified

Two-node chain, chain ID 777001, both nodes signers, `period 1`.

- **node 1**: `--miner.feerecipient 0x494f...5517` (R — not a signer, not
  unlocked, never signs)
- **node 2**: no flag, so fees follow its own signer address

Block header of a block sealed by node 1:

```
mixHash  = 0x000000000000000000000000 494f065c6178442b4ba89185e68b8790d25f5517
miner    = 0x0000000000000000000000000000000000000000
```

The padding is zero, the address is R, and the vote-target field stays zero.

Balances after interleaved transactions from both nodes, at height 44:

| | node 1 view | node 2 view |
|---|---|---|
| A (signer 1, fees → R) | 999,994.96 J | 999,994.96 J |
| B (signer 2, fees → self) | 1,000,000.833 J | 1,000,000.833 J |
| R (A's recipient) | 4.196403005886 J | 4.196403005886 J |

Identical to the wei on both nodes, which is the property that matters: each
node independently validated the other's blocks, including the ones carrying a
fee recipient it had no part in choosing.

Restarted both nodes and let them re-sync: heights converged (73/73), head block
hashes identical, R balance unchanged, no `invalid`/`mismatch`/`bad block` in
either log. Historical blocks carrying the field were re-validated from scratch.

Single-node run confirms the accounting: 5 transactions at 21,000 gas and
20 gwei — signer paid exactly 2,100,000 gwei, R received 2,095,042.77 gwei,
4,957.23 gwei burned as base fee.

---

## Interactions worth knowing

**`header.Difficulty == 0` makes `NewEVMBlockContext` treat `MixDigest` as the
post-merge randomness field.** Clique difficulty is always 1 or 2, so the two
uses cannot collide on a Clique chain. This is a property of the current code,
not a guarantee, and it is the first thing to re-check if Clique's difficulty
rules ever change.

**A chain with two or more signers will stall if only some of them run.** Not
related to this change: Clique requires the signer to wait
`len(signers)/2+1` blocks, so one live signer out of two cannot produce a chain
alone. It shows up as `signed recently, must wait for others`, which reads like
a bug and is not. Cost a false alarm during testing.

---

## What this does not do

- **Base fee is still burned.** Unchanged from before: `buyGas` destroys it
  before the tip is computed.
- **No local account is needed, but also no spending power is granted.** If the
  recipient is a contract, the J accrues there and only that contract's logic
  can move it. On a chain that still blocks value transfers, the accrued J can
  pay gas from that address but cannot be transferred out.
- **It is not a way to evade the transfer ban.** Fees are credited, not
  transferred; the recipient does not gain the ability to send value.
- **Nothing stops a signer from naming an address it does not control.** That is
  the same freedom a Bitcoin miner has, and the same consequence: the fees are
  gone. It is the operator's own loss, not a consensus problem.
