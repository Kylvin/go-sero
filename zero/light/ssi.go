package light

import (
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"

	"github.com/sero-cash/go-sero/common"
	"github.com/sero-cash/go-sero/zero/light/light_issi"
	"github.com/sero-cash/go-sero/zero/light/light_ref"
	"github.com/sero-cash/go-sero/zero/light/light_types"
	"github.com/sero-cash/go-sero/zero/txs/assets"
	"github.com/sero-cash/go-sero/zero/utils"

	"github.com/sero-cash/go-czero-import/keys"
	"github.com/sero-cash/go-sero/zero/localdb"
)

type SSI struct {
}

var SSI_Inst = SSI{}

func (self *SSI) GetBlocksInfo(start uint64, count uint64) (blocks []light_issi.Block, e error) {

	if bs, err := SRI_Inst.GetBlocksInfo(start, count); err != nil {
		e = err
		return
	} else {
		for _, b := range bs {
			block := light_issi.Block{}
			block.Num = b.Num
			block.Hash = b.Hash
			block.Nils = b.Nils
			for _, o := range b.Outs {
				block.Outs = append(
					block.Outs,
					light_issi.Out{
						o.Root,
						o.State.TxHash,
						*o.State.OS.ToPKr(),
					},
				)
			}
			blocks = append(blocks, block)
		}
	}

	return
}

func (self *SSI) Detail(roots []keys.Uint256, skr *keys.PKr) (douts []light_types.DOut, e error) {

	outs := []light_types.Out{}
	for _, r := range roots {
		if root := localdb.GetRoot(light_ref.Ref_inst.Bc.GetDB(), &r); root == nil {
			e = fmt.Errorf("SSI Detail Error for root %v", r)
			return
		} else {
			outs = append(outs, light_types.Out{r, *root})
		}
	}
	douts = SLI_Inst.DecOuts(outs, skr)

	return
}

var txMap sync.Map

func (self *SSI) GenTx(param *light_issi.GenTxParam) (hash keys.Uint256, e error) {
	log.Printf("genTx start")
	p := light_types.GenTxParam{}
	p.Gas = param.Gas
	p.GasPrice = *big.NewInt(0).SetUint64(param.GasPrice)
	p.From = param.From
	p.Outs = param.Outs

	roots := []keys.Uint256{}
	outs := []light_types.Out{}

	amounts := make(map[string]*big.Int)
	ticekts := make(map[keys.Uint256]keys.Uint256)
	for _, in := range param.Ins {
		roots = append(roots, in.Root)
		if root := localdb.GetRoot(light_ref.Ref_inst.Bc.GetDB(), &in.Root); root == nil {
			e = fmt.Errorf("SSI GenTx Error for root %v", in.Root)
			return
		} else {
			out := light_types.Out{in.Root, *root}
			dOuts := SLI_Inst.DecOuts([]light_types.Out{out}, &in.SKr)
			if len(dOuts) == 0 {
				e = fmt.Errorf("SSI GenTx Error for root %v", in.Root)
				return
			}
			oOut := dOuts[0]
			if oOut.Asset.Tkn != nil {
				currency := strings.Trim(string(oOut.Asset.Tkn.Currency[:]), string([]byte{0}))
				if amount, ok := amounts[currency]; ok {
					amount.Add(amount, oOut.Asset.Tkn.Value.ToIntRef())
				} else {
					amounts[currency] = oOut.Asset.Tkn.Value.ToIntRef()
				}

			}
			if oOut.Asset.Tkt != nil {
				ticekts[oOut.Asset.Tkt.Value] = oOut.Asset.Tkt.Category
			}
			outs = append(outs, light_types.Out{in.Root, *root})
		}
	}

	for _, out := range param.Outs {
		if out.Asset.Tkn != nil {
			currency := strings.Trim(string(out.Asset.Tkn.Currency[:]), string([]byte{0}))
			token := out.Asset.Tkn.Value.ToIntRef()
			if amount, ok := amounts[currency]; ok && amount.Cmp(token) >= 0 {
				amount.Sub(amount, token)
				if amount.Sign() == 0 {
					delete(amounts, currency)
				}
			} else {
				e = fmt.Errorf("SSI GenTx Error: balance is not enough")
				return
			}
		}
		if out.Asset.Tkt != nil {
			if value, ok := ticekts[out.Asset.Tkt.Value]; ok && value == out.Asset.Tkt.Category {
				delete(ticekts, out.Asset.Tkt.Value)
			} else {
				e = fmt.Errorf("SSI GenTx Erro: balance is not enough")
				return
			}
		}
	}

	fee := new(big.Int).Mul(new(big.Int).SetUint64(param.Gas), new(big.Int).SetUint64(param.GasPrice))
	if amount, ok := amounts["SERO"]; !ok || amount.Cmp(fee) < 0 {
		e = fmt.Errorf("SSI GenTx Error: sero amount < Fee")
		return
	} else {
		amount.Sub(amount, fee)
		if amount.Sign() == 0 {
			delete(amounts, "SERO")
		}
	}

	if len(amounts) > 0 || len(ticekts) > 0 {
		for currency, value := range amounts {
			p.Outs = append(p.Outs, light_types.GOut{PKr: p.From.PKr, Asset: assets.Asset{Tkn: &assets.Token{
				Currency: *common.BytesToHash(common.LeftPadBytes([]byte(currency), 32)).HashToUint256(),
				Value:    utils.U256(*value),
			}}})
		}
		for value, category := range ticekts {
			p.Outs = append(p.Outs, light_types.GOut{PKr: p.From.PKr, Asset: assets.Asset{Tkt: &assets.Ticket{
				Category: category,
				Value:    value,
			}}})
		}
	}

	wits := []light_types.Witness{}

	if wits, e = SRI_Inst.GetAnchor(roots); e != nil {
		return
	}

	for i := 0; i < len(wits); i++ {
		in := light_types.GIn{}
		in.SKr = param.Ins[i].SKr
		in.Out = outs[i]
		in.Witness = wits[i]
		p.Ins = append(p.Ins, in)
	}

	log.Printf("genTx ins : %v, outs : %v", len(p.Ins), len(p.Outs))
	if gtx, err := SLI_Inst.GenTx(&p); err != nil {
		e = err
		log.Printf("genTx error : %v", err)
		return
	} else {
		hash = gtx.Tx.ToHash()
		txMap.Store(hash, &gtx)
		log.Printf("genTx success hash: %s", common.Bytes2Hex(hash[:]))
	}

	return
}

func (self *SSI) GetTx(txhash keys.Uint256) (tx *light_types.GTx, e error) {
	if ld, ok := txMap.Load(txhash); !ok {
		e = fmt.Errorf("SSI GetTx Failed : %v", txhash)
	} else {
		if ld == nil {
			e = fmt.Errorf("SSI GetTx Nil : %v", txhash)
		} else {
			tx = ld.(*light_types.GTx)
		}
	}
	return
}
