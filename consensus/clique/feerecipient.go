// Copyright 2026 The Jouleverse Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the License,
// or (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package clique

import "math/big"

// FeeRecipientActivation is the block height from which a Clique block may name
// its own fee recipient.
//
// Unlike a chain-wide recipient, this needs no coordinated upgrade to be
// *useful*: a signer that has not upgraded produces blocks with the field zero
// and simply keeps its own fees, and those blocks remain valid. What the height
// gates is participation, not validity — above it a node that wants the feature
// can use it, below it nobody can, so historical blocks keep their exact meaning
// and are never reinterpreted.
//
// Zero means "active from genesis", which is what a freshly created chain wants.
var FeeRecipientActivation int64 = 0

// EnablesFeeRecipient reports whether a block at the given height may carry a
// fee recipient.
func EnablesFeeRecipient(number *big.Int) bool {
	if number == nil {
		return false
	}
	return number.Cmp(big.NewInt(FeeRecipientActivation)) >= 0
}
