import { expect } from "chai";
import { ethers } from "hardhat";
import { mineSolution } from "./helpers";

describe("AuryxToken economics", () => {
  it("raises difficulty (lowers target) when solves come faster than target time", async () => {
    const [m] = await ethers.getSigners();
    const t = await (await ethers.getContractFactory("TestAuryx")).deploy(); // easy genesis
    const before = await t.getMiningTarget();
    // mine one full retarget window (BLOCKS_PER_RETARGET = 16) quickly
    for (let i = 0; i < 16; i++) {
      const c = await t.getChallengeNumber();
      const tgt = await t.getMiningTarget();
      const { nonce, digest } = mineSolution(c, m.address, tgt);
      await t.mint(nonce, digest);
    }
    const after = await t.getMiningTarget();
    expect(after).to.be.lessThan(before); // harder
  });

  it("halves the reward at an era boundary", async () => {
    const [m] = await ethers.getSigners();
    const t = await (await ethers.getContractFactory("TestAuryx")).deploy(); // era = 4 solves
    expect(await t.getMiningReward()).to.equal(50n * 10n ** 18n);
    for (let i = 0; i < 4; i++) {
      const c = await t.getChallengeNumber();
      const tgt = await t.getMiningTarget();
      const { nonce, digest } = mineSolution(c, m.address, tgt);
      await t.mint(nonce, digest);
    }
    expect(await t.getMiningReward()).to.equal(25n * 10n ** 18n); // halved
  });

  it("keeps total supply within the cap", async () => {
    const t = await (await ethers.getContractFactory("AuryxToken")).deploy();
    expect(await t.totalSupply()).to.be.lessThanOrEqual(await t.MAX_SUPPLY());
  });
});
