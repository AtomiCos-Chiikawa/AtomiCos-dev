// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// Minimal slice of the cosmos/evm staking precompile interface (precompiles/staking/StakingI.sol).
interface IStakingDelegate {
    function delegate(
        address delegatorAddress,
        string memory validatorAddress,
        uint256 amount
    ) external returns (bool success);
}

/// @notice Observation-only contract for GHSA-7g4w-cg88-2cq2.
/// It delegates from its own address through the staking precompile and reports the
/// EVM-visible balance (StateDB) of this address right before and right after the call.
/// It does not move funds anywhere else; it is not an exploit.
contract VestingDelegateProbe {
    IStakingDelegate private constant STAKING =
        IStakingDelegate(0x0000000000000000000000000000000000000800);

    function probe(string calldata validator, uint256 amount)
        external
        returns (bool ok, uint256 balanceBefore, uint256 balanceAfter)
    {
        balanceBefore = address(this).balance;
        ok = STAKING.delegate(address(this), validator, amount);
        balanceAfter = address(this).balance;
    }
}
