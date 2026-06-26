import { expect } from "chai";
import { ethers } from "hardhat";

describe("AuryxToken metadata", () => {
  it("has correct name/symbol/decimals, zero start supply, 100M cap", async () => {
    const Auryx = await ethers.getContractFactory("AuryxToken");
    const t = await Auryx.deploy();
    expect(await t.name()).to.equal("Auryx");
    expect(await t.symbol()).to.equal("AURYX");
    expect(await t.decimals()).to.equal(18n);
    expect(await t.totalSupply()).to.equal(0n);
    expect(await t.MAX_SUPPLY()).to.equal(100_000_000n * 10n ** 18n);
  });
});
