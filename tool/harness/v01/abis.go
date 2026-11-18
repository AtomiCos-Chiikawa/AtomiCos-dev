// Method decoding stub for cosmos/evm v0.1.x (copied into evmd/atomicos by inject.sh).
// O2 is not observed in this version, so decodeMethod is never called with real input.

package atomicos

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
)

func decodeMethod(_ common.Address, input []byte) (string, map[string]string) {
	if len(input) < 4 {
		return "", nil
	}
	return "0x" + hex.EncodeToString(input[:4]), nil
}
