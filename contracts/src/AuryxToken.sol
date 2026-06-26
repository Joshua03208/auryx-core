// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @title Auryx — a fair-launch, mineable ERC-20 (EIP-918 style).
/// @notice Coins are created ONLY by submitting valid proof-of-work to `mint()`.
///         No premine, no owner, no admin mint. Difficulty retargets on-chain and
///         the reward halves each era, like Bitcoin. Binding `msg.sender` into the
///         hashed solution makes a found solution impossible to steal from the mempool.
contract AuryxToken is ERC20 {
    // ----- fixed economics -----
    uint256 public constant MAX_SUPPLY = 100_000_000 * 1e18;
    uint256 public constant INITIAL_REWARD = 50 * 1e18; // halves each era; 50*1M*2 = 100M
    uint256 public constant BLOCKS_PER_ERA = 1_000_000; // reward halves each era
    uint256 public constant TARGET_SECONDS_PER_SOLVE = 60;
    uint256 public constant BLOCKS_PER_RETARGET = 16; // snappier than Bitcoin so it converges fast

    // a digest is a valid solution if (uint256(digest) < miningTarget).
    // Higher target => easier. MAX_TARGET is the easiest ever allowed (the floor of
    // difficulty); GENESIS_TARGET is where difficulty STARTS — set so the first
    // blocks already take real work on real hardware (no instant-mint gush).
    uint256 public constant MIN_TARGET = 2 ** 16;
    uint256 public constant MAX_TARGET = 2 ** 252;
    // ~2**29 (536M) expected hashes per block => ~10s on a fast multi-core CPU at
    // genesis. Retargeting then climbs it toward the 60s target. No instant gush.
    uint256 public constant GENESIS_TARGET = 2 ** 227;

    // ----- mining state -----
    bytes32 public challengeNumber;
    uint256 public miningTarget;
    uint256 public epochCount; // total solves found
    uint256 public rewardEra; // halving era
    uint256 public retargetWindowStart; // timestamp the current retarget window began

    event Mint(address indexed from, uint256 reward, uint256 epochCount, bytes32 newChallengeNumber);

    constructor() ERC20("Auryx", "AURYX") {
        miningTarget = _genesisTarget();
        retargetWindowStart = block.timestamp;
        challengeNumber = keccak256(abi.encodePacked(blockhash(block.number - 1), address(this)));
    }

    /// @dev Override seam so tests can start at an easy genesis (fast to mine in JS);
    ///      production starts at GENESIS_TARGET so early mining isn't instant.
    function _genesisTarget() internal pure virtual returns (uint256) {
        return GENESIS_TARGET;
    }

    // ----- the mining entrypoint -----

    /// @notice Submit a proof-of-work solution. If valid, mints the era's reward
    ///         to the caller, then retargets difficulty and rolls a new challenge.
    /// @param nonce the value you searched for
    /// @param challengeDigest must equal keccak256(challengeNumber, msg.sender, nonce)
    function mint(uint256 nonce, bytes32 challengeDigest) external returns (bool) {
        bytes32 digest = keccak256(abi.encodePacked(challengeNumber, msg.sender, nonce));
        require(digest == challengeDigest, "wrong digest");
        require(uint256(digest) < miningTarget, "insufficient difficulty");

        uint256 reward = INITIAL_REWARD >> rewardEra;
        require(totalSupply() + reward <= MAX_SUPPLY, "cap reached");

        _mint(msg.sender, reward);
        _startNewEpoch();

        emit Mint(msg.sender, reward, epochCount, challengeNumber);
        return true;
    }

    // ----- EIP-918 read surface (what miners + the dApp consume) -----

    function getChallengeNumber() external view returns (bytes32) {
        return challengeNumber;
    }

    function getMiningTarget() external view returns (uint256) {
        return miningTarget;
    }

    function getMiningReward() external view returns (uint256) {
        return INITIAL_REWARD >> rewardEra;
    }

    function getMiningDifficulty() external view returns (uint256) {
        return MAX_TARGET / miningTarget;
    }

    // ----- internals -----

    function _startNewEpoch() internal {
        epochCount += 1;
        if (epochCount % _blocksPerEra() == 0) {
            rewardEra += 1;
        }
        if (epochCount % BLOCKS_PER_RETARGET == 0) {
            _reAdjustDifficulty();
        }
        challengeNumber = keccak256(abi.encodePacked(challengeNumber, msg.sender, blockhash(block.number - 1)));
    }

    /// @dev Move `miningTarget` toward the value that would have produced solves at
    ///      the target rate over the last window. Faster solving => smaller target
    ///      (harder); slower => larger (easier). Swings clamped to 2x per window.
    function _reAdjustDifficulty() internal {
        uint256 elapsed = block.timestamp - retargetWindowStart;
        uint256 expected = TARGET_SECONDS_PER_SOLVE * BLOCKS_PER_RETARGET;
        if (elapsed < expected / 2) elapsed = expected / 2;
        if (elapsed > expected * 2) elapsed = expected * 2;

        // divide before multiply: miningTarget can be ~2**252, so (target * elapsed)
        // would overflow uint256. Precision loss is negligible at this scale.
        uint256 newTarget = (miningTarget / expected) * elapsed;
        if (newTarget < MIN_TARGET) newTarget = MIN_TARGET;
        if (newTarget > MAX_TARGET) newTarget = MAX_TARGET;

        miningTarget = newTarget;
        retargetWindowStart = block.timestamp;
    }

    /// @dev Override seam so tests can use a tiny era; production stays at BLOCKS_PER_ERA.
    function _blocksPerEra() internal view virtual returns (uint256) {
        return BLOCKS_PER_ERA;
    }
}
