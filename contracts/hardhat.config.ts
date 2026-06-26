import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";
import * as dotenv from "dotenv";
dotenv.config();

const { BASE_SEPOLIA_RPC_URL, DEPLOYER_PRIVATE_KEY, BASESCAN_API_KEY } = process.env;

// Accept the key with or without a 0x prefix (MetaMask exports it without one).
const deployerKey = DEPLOYER_PRIVATE_KEY
  ? DEPLOYER_PRIVATE_KEY.startsWith("0x")
    ? DEPLOYER_PRIVATE_KEY
    : `0x${DEPLOYER_PRIVATE_KEY}`
  : undefined;

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.24",
    settings: { optimizer: { enabled: true, runs: 200 } },
  },
  paths: {
    sources: "./src",
  },
  networks: {
    baseSepolia: {
      url: BASE_SEPOLIA_RPC_URL || "https://sepolia.base.org",
      chainId: 84532,
      accounts: deployerKey ? [deployerKey] : [],
    },
  },
  // Etherscan API V2: one key, auto-routed by chain ID. Works for Base + Base
  // Sepolia (both are built into hardhat-verify's chain list).
  etherscan: {
    apiKey: BASESCAN_API_KEY || "",
  },
};
export default config;
