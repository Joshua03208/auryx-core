// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {AuryxToken} from "../AuryxToken.sol";

/// @dev Test-only subclass: tiny era (to exercise halving) and an EASY genesis
///      target (so the JS test miner finds solutions instantly). Never deployed in
///      production (the deploy script only deploys AuryxToken).
contract TestAuryx is AuryxToken {
    function _blocksPerEra() internal pure override returns (uint256) {
        return 4;
    }

    function _genesisTarget() internal pure override returns (uint256) {
        return MAX_TARGET; // easiest possible, so tests mine in microseconds
    }
}
