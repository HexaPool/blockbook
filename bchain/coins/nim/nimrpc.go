package nim

import (
	"blockbook/bchain"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/terorie/gimiq/networks"
	"math/big"
	"time"
)

// NimiqNet type specifies the type of Nimiq network
type NimiqNet uint8

const (
	// MainNet is production network
	MainNet NimiqNet = 42
	// TestNet is test network
	TestNet NimiqNet = 1
)

// Configuration represents json config file
type Configuration struct {
	CoinName             string `json:"coin_name"`
	CoinShortcut         string `json:"coin_shortcut"`
	RPCURL               string `json:"rpc_url"`
	RPCTimeout           int    `json:"rpc_timeout"`
	BlockAddressesToKeep int    `json:"block_addresses_to_keep"`
}

// NimiqRPC is an interface to JSON-RPC nim service.
type NimiqRPC struct {
	*bchain.BaseChain
	rpc         *rpc.Client
	timeout     time.Duration
	ChainConfig *Configuration
}

func NewNimiqRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	var err error
	var c Configuration
	err = json.Unmarshal(config, &c)
	if err != nil {
		return nil, errors.Annotatef(err, "Invalid configuration file")
	}
	// keep at least 100 mappings block->addresses to allow rollback
	if c.BlockAddressesToKeep < 100 {
		c.BlockAddressesToKeep = 100
	}
	rc, err := rpc.Dial(c.RPCURL)
	if err != nil {
		return nil, err
	}

	s := &NimiqRPC{
		BaseChain:   &bchain.BaseChain{},
		rpc:         rc,
		ChainConfig: &c,
		timeout:     time.Duration(c.RPCTimeout) * time.Second,
	}

	return s, nil
}

// Initialize initializes ethereum rpc interface
func (b *NimiqRPC) Initialize() error {
	genesis, err := b.GetBlock("", 1)
	if err != nil {
		return err
	}
	// parameters for getInfo request
	switch genesis.Hash {
	case hex.EncodeToString(networks.MainNet.Hash[:]):
		b.Testnet = false
		b.Network = "mainnet"
		break
	case hex.EncodeToString(networks.TestNet.Hash[:]):
		b.Testnet = true
		b.Network = "testnet"
		break
	case hex.EncodeToString(networks.DevNet.Hash[:]):
		b.Testnet = true
		b.Network = "devnet"
		break
	default:
		return errors.Errorf("Unknown network genesis %s", genesis.Hash)
	}
	glog.Info("rpc: block chain ", b.Network)

	return nil
}

// GetCoinName returns coin name
func (b *NimiqRPC) GetCoinName() string {
	return b.ChainConfig.CoinName
}

// GetSubversion returns empty string, Nimiq does not have subversion
func (b *NimiqRPC) GetSubversion() string {
	return ""
}

// GetChainInfo returns information about the connected backend
func (b *NimiqRPC) GetChainInfo() (*bchain.ChainInfo, error) {
	_, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()
	return nil, errors.New("not implemented")
}

// Shutdown cleans up rpc interface to Nimiq
func (b *NimiqRPC) Shutdown(ctx context.Context) error {
	if b.rpc != nil {
		b.rpc.Close()
	}
	glog.Info("rpc: shutdown")
	return nil
}

// GetBestBlockHash returns hash of the tip of the best-block-chain
func (b *NimiqRPC) GetBestBlockHash() (string, error) {
	headNum, err := b.GetBestBlockHeight()
	if err != nil {
		return "", err
	}

	ctx, _ := context.WithTimeout(context.Background(), b.timeout)

	var block *rpcHeader
	err = b.rpc.CallContext(ctx, &block, "getBlockByNumber", headNum)
	if err != nil {
		return "", err
	} else if block == nil {
		return "", bchain.ErrBlockNotFound
	}

	return block.Hash, nil
}

// GetBestBlockHeight returns height of the tip of the best-block-chain
func (b *NimiqRPC) GetBestBlockHeight() (uint32, error) {
	var ctx context.Context
	ctx, _ = context.WithTimeout(context.Background(), b.timeout)

	var headNum uint32
	err := b.rpc.CallContext(ctx, &headNum, "blockNumber")
	if err != nil {
		return 0, err
	}

	return headNum, nil
}

// GetBlockHash returns hash of block in best-block-chain at given height
func (b *NimiqRPC) GetBlockHash(height uint32) (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), b.timeout)

	var block *rpcHeader
	err := b.rpc.CallContext(ctx, &block, "getBlockByNumber", height)
	if err != nil {
		return "", err
	} else if block == nil {
		return "", bchain.ErrBlockNotFound
	}

	return block.Hash, nil
}

// GetBlockHeader returns header of block with given hash
func (b *NimiqRPC) GetBlockHeader(hash string) (*bchain.BlockHeader, error) {
	ctx, _ := context.WithTimeout(context.Background(), b.timeout)

	var block *rpcHeader
	err := b.rpc.CallContext(ctx, &block, "getBlockByHash", hash)
	if err != nil {
		return nil, err
	} else if block == nil {
		return nil, bchain.ErrBlockNotFound
	}

	h := b.nimHeaderToBlockHeader(block)

	return h, nil
}

// GetBlock returns block with given hash or height, hash has precedence if both passed
func (b *NimiqRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var raw json.RawMessage
	var err error

	ctx, _ := context.WithTimeout(context.Background(), b.timeout)

	if hash != "" {
		err = b.rpc.CallContext(ctx, &raw, "getBlockByHash", hash, true)
	} else {
		err = b.rpc.CallContext(ctx, &raw, "getBlockByNumber", height, true)
	}
	if err != nil {
		return nil, err
	}

	var head rpcHeader
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, errors.Annotatef(err, "hash %v, height %v", hash, height)
	}

	var body rpcBlock
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.Annotatef(err, "hash %v, height %v", hash, height)
	}

	bbh := b.nimHeaderToBlockHeader(&head)

	btxs := make([]bchain.Tx, len(body.Txs))
	for i, tx := range body.Txs {
		btxs[i] = *b.nimTxToTx(&tx)
	}

	bbk := &bchain.Block{
		BlockHeader: *bbh,
		Txs:         btxs,
	}
	return bbk, nil
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *NimiqRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	var raw json.RawMessage
	var err error

	ctx, _ := context.WithTimeout(context.Background(), b.timeout)

	err = b.rpc.CallContext(ctx, &raw, "getBlockByHash", hash, false)
	if err != nil {
		return nil, err
	}

	var head rpcHeader
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}

	var body rpcLightBlock
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}

	bbh := b.nimHeaderToBlockHeader(&head)

	bbk := &bchain.BlockInfo{
		BlockHeader: *bbh,
		Difficulty:  json.Number(head.Difficulty),
		Nonce:       json.Number(head.Nonce),
		Txids:       body.Txs,
	}
	return bbk, nil
}

// GetTransactionForMempool returns a transaction by the transaction ID.
// It could be optimized for mempool, i.e. without block time and confirmations
func (b *NimiqRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return nil, errors.New("GetTransactionForMempool: not supported")
}

// GetTransaction returns a transaction by the transaction ID.
func (b *NimiqRPC) GetTransaction(txid string) (*bchain.Tx, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()
	var tx *rpcTx
	err := b.rpc.CallContext(ctx, &tx, "getTransactionByHash", txid)
	if err != nil {
		return nil, err
	} else if tx == nil {
		return nil, bchain.ErrTxNotFound
	}

	return b.nimTxToTx(tx), nil
}

// GetTransactionSpecific returns json as returned by backend, with all coin specific data
func (b *NimiqRPC) GetTransactionSpecific(tx *bchain.Tx) (json.RawMessage, error) {
	return nil, errors.New("GetTransactionSpecific: not supported")
}

// GetMempool returns transactions in mempool
func (b *NimiqRPC) GetMempool() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()
	var txHashes []string
	err := b.rpc.CallContext(ctx, &txHashes, "mempoolContent", false)
	if err != nil {
		return nil, err
	}
	return txHashes, nil
}

// EstimateFee returns fee estimation
func (b *NimiqRPC) EstimateFee(blocks int) (big.Int, error) {
	return *big.NewInt(0), errors.New("GetMempoolEntry: not supported")
}

// EstimateSmartFee returns fee estimation
func (b *NimiqRPC) EstimateSmartFee(blocks int, conservative bool) (big.Int, error) {
	return *big.NewInt(0), errors.New("EstimateSmartFee: not supported")
}

// SendRawTransaction sends raw transaction
func (b *NimiqRPC) SendRawTransaction(hex string) (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), b.timeout)
	var txHash string
	err := b.rpc.CallContext(ctx, &txHash, "sendRawTransaction", hex)
	if err != nil {
		return "", err
	}
	return txHash, nil
}

// ResyncMempool gets mempool transactions and maps output scripts to transactions.
// ResyncMempool is not reentrant, it should be called from a single thread.
// Return value is number of transactions in mempool
func (b *NimiqRPC) ResyncMempool(onNewTxAddr bchain.OnNewTxAddrFunc) (int, error) {
	return 0, errors.New("not implemented")
}

// GetMempoolTransactions returns slice of mempool transactions for given address
func (b *NimiqRPC) GetMempoolTransactions(address string) ([]bchain.Outpoint, error) {
	return nil, errors.New("GetMempoolTransactions: not supported")
}

// GetMempoolTransactionsForAddrDesc returns slice of mempool transactions for given address descriptor
func (b *NimiqRPC) GetMempoolTransactionsForAddrDesc(addrDesc bchain.AddressDescriptor) ([]bchain.Outpoint, error) {
	return nil, errors.New("GetMempoolTransactionsForAddrDesc: not supported")
}

// GetMempoolEntry is not supported by Nimiq
func (b *NimiqRPC) GetMempoolEntry(txid string) (*bchain.MempoolEntry, error) {
	return nil, errors.New("GetMempoolEntry: not supported")
}

// GetChainParser returns Nimiq BlockChainParser
func (b *NimiqRPC) GetChainParser() bchain.BlockChainParser {
	return b.Parser
}
