import { ethers, network } from "hardhat";

async function main() {
  const Auryx = await ethers.getContractFactory("AuryxToken");
  const token = await Auryx.deploy();
  await token.waitForDeployment();
  const addr = await token.getAddress();
  console.log(`AuryxToken deployed to ${addr} on ${network.name}`);
  console.log(`Verify: npx hardhat verify --network ${network.name} ${addr}`);
}

main().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
