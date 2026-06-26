package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Minimal ABI: only the functions the miner needs from AuryxToken.
const auryxABI = `[
  {"inputs":[],"name":"getChallengeNumber","outputs":[{"type":"bytes32"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"getMiningTarget","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"getMiningReward","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"type":"address"}],"name":"balanceOf","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"type":"uint256"},{"type":"bytes32"}],"name":"mint","outputs":[{"type":"bool"}],"stateMutability":"nonpayable","type":"function"}
]`

// Chain wraps a JSON-RPC connection + the bound AuryxToken contract + the
// miner's signing key (its wallet receives rewards and pays gas).
type Chain struct {
	client  *ethclient.Client
	bound   *bind.BoundContract
	key     *ecdsa.PrivateKey
	from    common.Address
	chainID *big.Int
}

func NewChain(rpcURL, contractAddr, privHex string) (*Chain, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	parsed, err := abi.JSON(strings.NewReader(auryxABI))
	if err != nil {
		return nil, err
	}
	addr := common.HexToAddress(contractAddr)
	bound := bind.NewBoundContract(addr, parsed, client, client, client)
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("bad private key: %w", err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}
	return &Chain{client: client, bound: bound, key: key, from: from, chainID: chainID}, nil
}

func (c *Chain) From() common.Address { return c.from }

func (c *Chain) callUint(ctx context.Context, method string, args ...interface{}) (*big.Int, error) {
	var out []interface{}
	if err := c.bound.Call(&bind.CallOpts{Context: ctx}, &out, method, args...); err != nil {
		return nil, err
	}
	return out[0].(*big.Int), nil
}

func (c *Chain) Challenge(ctx context.Context) ([32]byte, error) {
	var out []interface{}
	if err := c.bound.Call(&bind.CallOpts{Context: ctx}, &out, "getChallengeNumber"); err != nil {
		return [32]byte{}, err
	}
	return out[0].([32]byte), nil
}

func (c *Chain) Target(ctx context.Context) (*big.Int, error) { return c.callUint(ctx, "getMiningTarget") }
func (c *Chain) Reward(ctx context.Context) (*big.Int, error) { return c.callUint(ctx, "getMiningReward") }
func (c *Chain) Balance(ctx context.Context, who common.Address) (*big.Int, error) {
	return c.callUint(ctx, "balanceOf", who)
}

func (c *Chain) SubmitMint(ctx context.Context, nonce *big.Int, digest [32]byte) (*types.Transaction, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(c.key, c.chainID)
	if err != nil {
		return nil, err
	}
	opts.Context = ctx
	return c.bound.Transact(opts, "mint", nonce, digest)
}

// WaitMined blocks until the tx is mined; returns true if it succeeded.
func (c *Chain) WaitMined(ctx context.Context, tx *types.Transaction) (bool, error) {
	rcpt, err := bind.WaitMined(ctx, c.client, tx)
	if err != nil {
		return false, err
	}
	return rcpt.Status == types.ReceiptStatusSuccessful, nil
}
