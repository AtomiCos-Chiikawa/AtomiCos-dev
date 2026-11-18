// Method decoding for precompile calls (copied into evmd/atomicos by inject.sh).
// Used by v0.7.x and v0.5-era trees, whose staking and distribution packages export ABI.

package atomicos

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/precompiles/staking"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var knownABIs = map[common.Address]*abi.ABI{
	common.HexToAddress(evmtypes.StakingPrecompileAddress):      &staking.ABI,
	common.HexToAddress(evmtypes.DistributionPrecompileAddress): &distribution.ABI,
}

// decodeMethod names the called method and its arguments; unknown precompiles get the selector.
func decodeMethod(precompile common.Address, input []byte) (string, map[string]string) {
	if len(input) < 4 {
		return "", nil
	}
	a, ok := knownABIs[precompile]
	if !ok {
		return "0x" + hex.EncodeToString(input[:4]), nil
	}
	m, err := a.MethodById(input[:4])
	if err != nil {
		return "0x" + hex.EncodeToString(input[:4]), nil
	}
	vals, err := m.Inputs.Unpack(input[4:])
	if err != nil {
		return m.Name, map[string]string{"_error": err.Error()}
	}
	args := make(map[string]string, len(vals))
	for i, in := range m.Inputs {
		if i >= len(vals) {
			break
		}
		switch x := vals[i].(type) {
		case common.Address:
			args[in.Name] = hexAddr(x)
		default:
			args[in.Name] = fmt.Sprint(x)
		}
	}
	return m.Name, args
}
