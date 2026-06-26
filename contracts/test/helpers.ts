// Off-chain PoW solver used by tests: brute-forces a nonce whose
// keccak256(challenge, miner, nonce) digest is below `target`.
import { ethers } from "hardhat";

export function mineSolution(challenge: string, miner: string, target: bigint) {
  for (let nonce = 0n; ; nonce++) {
    const digest = ethers.solidityPackedKeccak256(
      ["bytes32", "address", "uint256"],
      [challenge, miner, nonce]
    );
    if (BigInt(digest) < target) return { nonce, digest };
  }
}
