# AURYX

A fair-launch, **mineable** ERC-20 token on **Base**. Every coin is earned by real
proof-of-work — no premine, no admin mint, no pause. Difficulty retargets on-chain
and the reward halves over time, like Bitcoin's early days. It works in MetaMask.

> **Status:** currently on the **Base Sepolia testnet**. These coins are for
> testing, are free to mine, and have **no monetary value**. A mainnet launch is
> gated on a security audit and legal review.

This repo holds the two things you need to verify and mine AURYX: the **contract**
and the **miner**.

## Token

| | |
|---|---|
| Name / symbol | Auryx / **AURYX** |
| Decimals | 18 |
| Max supply | 100,000,000 (hard cap) |
| Premine | none (fair launch) |
| Mining | keccak256 proof-of-work, on-chain difficulty retarget + halving |
| Network | Base Sepolia (chain id 84532) |
| Contract | [`0x619Ab437232f58fd0FC7606b98BB2D4948734750`](https://sepolia.basescan.org/address/0x619Ab437232f58fd0FC7606b98BB2D4948734750#code) (verified) |

## How mining works

The supply starts at **0** and only grows through mining:

1. The contract publishes a **challenge** and a **target** (the difficulty).
2. Your miner searches for a `nonce` where `keccak256(challenge, yourAddress, nonce)`
   is **below the target**. Binding your address into the hash means a found
   solution can only ever pay **you** — it can't be stolen from the mempool.
3. You submit the nonce. The contract **re-checks it on-chain** and, if valid,
   **mints the block reward** to your wallet, then rolls a new challenge.
4. The contract **retargets difficulty** (more miners → harder) and **halves**
   the reward each era.

The difficulty is enforced **by the contract**, not the miner — so a modified
miner can't cheat; invalid solutions are simply rejected. That's why the miner can
be fully open source.

## Mine it

You need a wallet with a little **Base Sepolia test ETH** for gas (free from a
Base Sepolia faucet). That same wallet receives your mined AURYX.

**Download a prebuilt miner** from the [Releases](../../releases) page, or build it:

```bash
cd miner
go build -o auryx-miner .        # add .exe on Windows
./auryx-miner
```

First run asks for your mining wallet's private key (saved locally, never sent
anywhere but to sign your own transactions). The RPC + contract are baked in.
Pick **Start mining**, choose your CPU cores, and leave it running.

> Write your own miner if you like — the contract's mining interface is the
> public EIP-918 pattern (`getChallengeNumber()`, `getMiningTarget()`, `mint()`).

## Contract

`contracts/` is a Hardhat project (Solidity + TypeScript). The source is verified
on BaseScan, so you can read every rule the coin enforces.

```bash
cd contracts
npm install
npm test
```

## Why it's trustworthy

- **Fair launch, no premine** — the creator mines on the same terms as everyone.
- **No admin powers** — no owner mint, no pause; it can't be rugged.
- **Verified + open** — read the contract on BaseScan and the miner here.

---

*Not affiliated with Coinbase or Base. Testnet token, no monetary value.*
