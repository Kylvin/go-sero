package ethapi

import (
	"context"
	"math/big"

	"github.com/sero-cash/go-sero/zero/exchange"

	"github.com/sero-cash/go-sero/common"

	"github.com/sero-cash/go-czero-import/keys"
	"github.com/sero-cash/go-sero/common/hexutil"
	"github.com/sero-cash/go-sero/zero/light/light_types"
)

type PublicExchangeAPI struct {
	b Backend
}

func (s *PublicExchangeAPI) GetPkNumber(ctx context.Context, pk *keys.Uint512) (uint64, error) {
	return s.b.GetPkNumber(*pk)
}

func (s *PublicExchangeAPI) GetPkr(ctx context.Context, address *keys.Uint512, index *keys.Uint256) (pkr keys.PKr, e error) {
	return s.b.GetPkr(address, index)
}

func (s *PublicExchangeAPI) GetBalances(ctx context.Context, address keys.Uint512) (balances map[string]*big.Int) {
	return s.b.GetBalances(address)
}

type Big big.Int

func (b Big) MarshalJSON() ([]byte, error) {
	i := big.Int(b)
	return i.MarshalText()
}

// UnmarshalJSON implements json.Unmarshaler.
func (b *Big) UnmarshalJSON(input []byte) error {
	if isString(input) {
		input = input[1 : len(input)-1]
	}
	i := big.Int{}
	if e := i.UnmarshalText(input); e != nil {
		return e
	} else {
		*b = Big(i)
		return nil
	}
}

func isString(input []byte) bool {
	return len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"'
}

type ReceptionArgs struct {
	Addr     keys.PKr
	Currency string
	Value    *Big
}

type GenTxArgs struct {
	From       keys.Uint512
	Receptions []ReceptionArgs
	Gas        uint64
	GasPrice   uint64
	Roots      []keys.Uint256
}

func (args GenTxArgs) toTxParam() exchange.TxParam {
	receptions := []exchange.Reception{}
	for _, rec := range args.Receptions {
		receptions = append(receptions, exchange.Reception{
			rec.Addr,
			rec.Currency,
			(*big.Int)(rec.Value),
		})
	}
	return exchange.TxParam{args.From, receptions, args.Gas, args.GasPrice, args.Roots}
}

func (s *PublicExchangeAPI) GenTx(ctx context.Context, param GenTxArgs) (*light_types.GenTxParam, error) {
	return s.b.GenTx(param.toTxParam())
}

func (s *PublicExchangeAPI) GenTxWithSign(ctx context.Context, param GenTxArgs) (*light_types.GTx, error) {
	tx, e := s.b.GenTxWithSign(param.toTxParam())
	return tx, e
}

type Record struct {
	Pkr      keys.PKr
	Root     keys.Uint256
	TxHash   keys.Uint256
	Nil      keys.Uint256
	Num      uint64
	Currency string
	Value    *big.Int
}

func (s *PublicExchangeAPI) GetRecords(ctx context.Context, address hexutil.Bytes, begin, end uint64) (records []Record, err error) {

	utxos, err := s.b.GetRecords(address, begin, end)
	if err != nil {
		return
	}
	for _, utxo := range utxos {
		if utxo.Asset.Tkn != nil {
			records = append(records, Record{Pkr: utxo.Pkr, Root: utxo.Root, TxHash: utxo.TxHash, Nil: utxo.Nil, Num: utxo.Num, Currency: common.BytesToString(utxo.Asset.Tkn.Currency[:]), Value: utxo.Asset.Tkn.Value.ToIntRef()})
		}
	}
	return
}

func (s *PublicExchangeAPI) CommitTx(ctx context.Context, args *light_types.GTx) error {
	return s.b.CommitTx(args)
}