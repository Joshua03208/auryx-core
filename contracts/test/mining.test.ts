import { expect } from "chai";
import { ethers } from "hardhat";
import { mineSolution } from "./helpers";

describe("AuryxToken mining surface", () => {
  it("exposes a non-zero genesis challenge, target, and initial reward", async () => {
    const t = await (await ethers.getContractFactory("AuryxToken")).deploy();
    expect(await t.getChallengeNumber()).to.not.equal(ethers.ZeroHash);
    expect(await t.getMiningTarget()).to.be.greaterThan(0n);
    expect(await t.getMiningReward()).to.equal(50n * 10n ** 18n);
    expect(await t.epochCount()).to.equal(0n);
  });
});

describe("AuryxToken mint()", () => {
  // Mine against TestAuryx (easy genesis) so the JS solver is instant.
  it("mints the reward to a valid solver and rolls the challenge", async () => {
    const [miner] = await ethers.getSigners();
    const t = await (await ethers.getContractFactory("TestAuryx")).deploy();

    const challenge = await t.getChallengeNumber();
    const target = await t.getMiningTarget();
    const { nonce, digest } = mineSolution(challenge, miner.address, target);

    await expect(t.mint(nonce, digest)).to.emit(t, "Mint");
    expect(await t.balanceOf(miner.address)).to.equal(50n * 10n ** 18n);
    expect(await t.totalSupply()).to.equal(50n * 10n ** 18n);
    expect(await t.epochCount()).to.equal(1n);
    expect(await t.getChallengeNumber()).to.not.equal(challenge);
  });

  it("rejects a solution that does not match the digest", async () => {
    const t = await (await ethers.getContractFactory("AuryxToken")).deploy();
    await expect(t.mint(123n, ethers.ZeroHash)).to.be.revertedWith("wrong digest");
  });

  it("prevents stealing another miner's solution (msg.sender bound in the hash)", async () => {
    const [alice, bob] = await ethers.getSigners();
    const t = await (await ethers.getContractFactory("TestAuryx")).deploy();
    const challenge = await t.getChallengeNumber();
    const target = await t.getMiningTarget();
    // Alice mines a solution for HER address
    const { nonce, digest } = mineSolution(challenge, alice.address, target);
    // Bob submits it -> contract recomputes with Bob's address -> mismatch
    await expect(t.connect(bob).mint(nonce, digest)).to.be.revertedWith("wrong digest");
    // Alice can submit it
    await expect(t.connect(alice).mint(nonce, digest)).to.emit(t, "Mint");
  });
});
