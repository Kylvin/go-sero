// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"math/big"
	"sync/atomic"

	"container/heap"
	"io"
	"strings"

	"github.com/sero-cash/go-sero/common"
	"github.com/sero-cash/go-sero/common/hexutil"
	"github.com/sero-cash/go-sero/rlp"
	zstx "github.com/sero-cash/go-sero/zero/txs/stx"
	ztx "github.com/sero-cash/go-sero/zero/txs/tx"
	"github.com/sero-cash/go-sero/zero/utils"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
}

type txdata struct {
	Price    *big.Int `json:"gasPrice" gencodec:"required"`
	Payload  []byte   `json:"input"    gencodec:"required"`
	Currency string   `json:"currency"    gencodec:"required"`
	Stxt     *zstx.T  `json:"stxt"    gencodec:"required"`
}

type txdataMarshaling struct {
	Price   *hexutil.Big
	Payload hexutil.Bytes
	Stxt    *zstx.T
}

func NewTransaction(gasPrice *big.Int, data []byte, currency string) *Transaction {
	return newTransaction(gasPrice, data, currency)
}

func NewTxt(to *common.Address, value *big.Int, gasPrice *big.Int, gas uint64, z ztx.OutType, currency string) *ztx.T {

	outDatas := []ztx.Out{}
	if to != nil {
		outData := ztx.Out{
			Addr:  *to.ToUint512(),
			Value: *utils.U256(*value).ToRef(),
			Z:     z,
		}
		outDatas = []ztx.Out{outData}
	}
	fee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gas))

	sero := common.BytesToHash(common.LeftPadBytes([]byte("sero"), 32))
	if currency == "sero" {
		ctx := ztx.CTx{
			Currency: *(sero.HashToUint256()),
			Fee:      utils.U256(*fee),
			Outs:     outDatas,
		}
		tx := &ztx.T{
			CTxs: []ztx.CTx{ctx},
		}
		return tx
	} else {
		ctxFee := ztx.CTx{
			Currency: *(sero.HashToUint256()),
			Fee:      utils.U256(*fee),
		}
		ctxAmount := ztx.CTx{
			Currency: *(common.BytesToHash(common.LeftPadBytes([]byte(currency), 32)).HashToUint256()),
			Outs:     outDatas,
		}
		tx := &ztx.T{
			CTxs: []ztx.CTx{ctxFee, ctxAmount},
		}
		return tx
	}

}

func newTransaction(gasPrice *big.Int, data []byte, currency string) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		Payload:  data,
		Price:    new(big.Int),
		Currency: currency,
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &Transaction{data: d}
}

func (tx *Transaction) Value() *big.Int {
	for _, desc_o := range tx.data.Stxt.Desc_Os {
		for _, out := range desc_o.Outs {
			return out.Value.ToIntRef()
		}
	}
	return big.NewInt(0)
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	//hash := tx.Hash()
	data := tx.data
	//data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}
	*tx = Transaction{data: dec}
	return nil
}

func (tx *Transaction) Data() []byte { return common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64 {
	fee := utils.NewU256(0)
	for _, desc_o := range tx.data.Stxt.Desc_Os {
		fee.AddU(&desc_o.Fee)
	}
	price := tx.data.Price
	return new(big.Int).Div(fee.ToIntRef(), price).Uint64()
}
func (tx *Transaction) GasPrice() *big.Int { return new(big.Int).Set(tx.data.Price) }

func (tx *Transaction) GetZZSTX() *zstx.T {
	return tx.data.Stxt
}

func (tx *Transaction) To() *common.Address {
	for _, desc_o := range tx.data.Stxt.Desc_Os {
		for _, out := range desc_o.Outs {
			var addr common.Address
			copy(addr[:], out.Addr[:])
			return &addr
		}
	}
	return nil
}

func (tx *Transaction) Stxt() *zstx.T {
	return tx.data.Stxt
}

func (tx *Transaction) From() common.Address {
	var addr common.Address
	copy(addr[:], tx.data.Stxt.From[:])
	return addr
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := tx.data.Stxt.ToHash()
	var hashv common.Hash
	copy(hashv[:], v[:])
	tx.hash.Store(hashv)
	return hashv
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlpData := []interface{}{tx.data.Currency, tx.data.Payload, tx.data.Price}
	rlp.Encode(&c, rlpData)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

func (tx *Transaction) Currency() string {
	if strings.TrimSpace(tx.data.Currency) == "" {
		return "sero"
	}
	return tx.data.Currency
}

// AsMessage returns the transaction as a core.Message.
func (tx *Transaction) AsMessage() (Message, error) {
	msg := Message{
		from:       tx.From(),
		gasLimit:   tx.Gas(),
		gasPrice:   new(big.Int).Set(tx.data.Price),
		to:         tx.To(),
		data:       tx.data.Payload,
		checkNonce: true,
		currency:   tx.Currency(),
		amount:     tx.Value(),
	}

	return msg, nil
}

func (tx *Transaction) WithEncrypt(stxt *zstx.T) (*Transaction, error) {
	cpy := &Transaction{data: tx.data}
	cpy.data.Stxt = stxt
	return cpy, nil
}

func (tx *Transaction) RawEncrptyValue() *zstx.T {
	return tx.data.Stxt
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].data.Price.Cmp(s[j].data.Price) > 0 }
func (s TxByPrice) Swap(i, j int) {
	if i < 0 || j < 0 {
		return
	}
	s[i], s[j] = s[j], s[i]
}

func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

func (s *TxByPrice) Pop() interface{} {
	if s.Len() < 1 {
		return nil
	}
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByPriceAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByPrice struct {
	txs   map[common.Address]Transactions // Per account nonce-sorted list of transactions
	heads TxByPrice                       // Next transaction for each unique account (price heap)
}

// NewTransactionsByPriceAndNonce creates a transaction set that can retrieve
// price sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByPrice(txs Transactions) *TransactionsByPrice {
	// Initialize a price based heap with the head transactions
	heads := make(TxByPrice, 0, len(txs))
	for _, tx := range txs {
		heads = append(heads, tx)
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &TransactionsByPrice{
		heads: heads,
	}
}

//// Peek returns the next transaction by price.
func (t *TransactionsByPrice) Peek() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByPrice) Shift() {
	acc := t.heads[0].From()
	if txs, ok := t.txs[acc]; ok && len(txs) > 0 {
		t.heads[0], t.txs[acc] = txs[0], txs[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByPrice) Pop() *Transaction {
	transaction := heap.Pop(&t.heads)
	if transaction == nil {
		return nil
	}
	return transaction.(*Transaction)
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	currency   string
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	data       []byte
	checkNonce bool
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, currency string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		currency:   currency,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		data:       data,
		checkNonce: checkNonce,
	}
}

func (m Message) From() common.Address { return m.from }
func (m Message) To() *common.Address  { return m.to }
func (m Message) GasPrice() *big.Int   { return m.gasPrice }
func (m Message) Value() *big.Int      { return m.amount }
func (m Message) Gas() uint64          { return m.gasLimit }
func (m Message) Nonce() uint64        { return m.nonce }
func (m Message) Data() []byte         { return m.data }
func (m Message) CheckNonce() bool     { return m.checkNonce }
func (m Message) Currency() string     { return m.currency }
