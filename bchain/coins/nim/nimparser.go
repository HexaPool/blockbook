package nim

import (
	"blockbook/bchain"
	"math/big"
)

// NimiqTypeAddressDescriptorLen - in case of NimiqType, the AddressDescriptor has fixed length
const NimiqTypeAddressDescriptorLen = 20

// NimiqAmountDecimalPoint defines number of decimal points in Nimiq amounts
const EtherAmountDecimalPoint = 5

// NimiqParser handle
type NimiqParser struct {
	*bchain.BaseParser
}

// NewNimiqParser returns new NimiqParser instance
func NewEthereumParser(b int) *NimiqParser {
	return &NimiqParser{&bchain.BaseParser{
		BlockAddressesToKeep: b,
		AmountDecimalPoint:   EtherAmountDecimalPoint,
	}}
}

type rpcHeader struct {
	Number       int64  `json:"number"`
	Hash         string `json:"hash"`
	PoW          string `json:"pow"`
	ParentHash   string `json:"parentHash"`
	Nonce        string `json:"nonce"`
	BodyHash     string `json:"bodyHash"`
	AccountsHash string `json:"accountsHash"`
	Miner        string `json:"miner"`
	MinerAddress string `json:"minerAddress"`
	Difficulty   string `json:"difficulty"`
	ExtraData    string `json:"extraData"`
	Size         uint64 `json:"size"`
	Time         int64  `json:"timestamp"`
}

type rpcLightBlock struct {
	Txs []string `json:"transactions"`
}

type rpcBlock struct {
	Txs []rpcTx `json:"transactions"`
}

type rpcTx struct {
	Hash             string `json:"hash"`
	BlockHash        string `json:"blockHash"`
	Timestamp        uint64 `json:"timestamp"`
	Confirmations    int    `json:"confirmations"`
	TransactionIndex int    `json:"transactionIndex"`
	From             string `json:"from"`
	FromAddress      string `json:"fromAddress"`
	To               string `json:"to"`
	ToAddress        string `json:"toAddress"`
	Value            uint64 `json:"value"`
	Fee              uint64 `json:"fee"`
}

func (b *NimiqRPC) nimHeaderToBlockHeader(block *rpcHeader) *bchain.BlockHeader {
	return &bchain.BlockHeader{
		Hash:   block.Hash,
		Prev:   block.ParentHash,
		Height: uint32(block.Number),
		Size:   int(block.Size),
		Time:   block.Time,
	}
}

func (b *NimiqRPC) nimTxToTx(tx *rpcTx) *bchain.Tx {
	btx := &bchain.Tx{
		Txid: tx.Hash,
		Vin: []bchain.Vin{
			{Addresses: []string{tx.From}},
		},
		Vout: []bchain.Vout{
			{
				N:        0,
				ValueSat: *big.NewInt(int64(tx.Value)),
				ScriptPubKey: bchain.ScriptPubKey{
					Addresses: []string{tx.To},
				},
			},
		},
	}
	return btx
}
