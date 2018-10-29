package restful

import (
	"fmt"

	"math/big"

	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/ethereum/go-ethereum/common"
)

/*
Tx 委托状态查询
Get /\<delegater\>/channel/<channel_address>
示例返回
```json
{
  "status":1,  //1 表示委托失败,2委托成功,但是余额不足,3委托成功,并且余额充足。
  "error":"错误描述", //正常情况下应该为空
  "channel_address":"0x5B3F0E96E45e1e4351F6460feBfB6007af25FBB0",
  "update_transfer":{
    "nonce":32,
    "transferred_amount":1800000000000000,
    "locksroot":" 0x447b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89",
    "extra_hash":" 0x557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89",
    "closing_signature":" 0x557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e8927",
    "non_closing_signature":" 0x557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e8927"
  },
  "withdraws":[
    {
      "locked_encoded":"0x00000033333333333333333333333333333333333333333",
      "merkle_proof":"0x3333333333333333333333333333",
      "secret":"0x333333333333333333333333333333333333333",
    },
    {
      "locked_encoded":"0x00000033333333333333333333333333333333333333333",
      "merkle_proof":"0x3333333333333333333333333333",
      "secret":"0x333333333333333333333333333333333333333",
    }
  ]
}
*/
func Tx(w rest.ResponseWriter, r *rest.Request) {
	var err2 error
	delegaterStr := r.PathParam("delegater")
	channel := r.PathParam("channel")
	delegater := common.HexToAddress(delegaterStr)
	if delegater == utils.EmptyAddress || channel == utils.EmptyAddress.String() {
		err2 = w.WriteJson(&txResponse{
			Status: delegateError,
			Error:  "arg error",
		})
		return
	}
	channelIdentifier := common.HexToHash(channel)
	d := db.DelegatetGet(channelIdentifier, delegater)
	res := &txResponse{
		Delegate: d,
	}
	if d.Content != nil {
		if db.AccountIsBalanceEnough(delegater) {
			res.Status = delegateSuccess
		} else {
			res.Status = delegateSuccessButNotEnoughBalance
		}
	} else {
		res.Status = delegateError
		res.Error = "no delegate"
	}

	err2 = w.WriteJson(res)
	if err2 != nil {
		log.Error(fmt.Sprintf("write json err %s", err2))
	}
}

type txResponse struct {
	Status   int
	Error    string
	Delegate *models.Delegate
}

/*
Fee 费用查询
Get /\<delegater\>/fee
```json
{
  "avaiable_smt":30900000000000000000000, //已经转账 smt 数量
  "need_smt":300000000000000, //还差多少个
}
```
*/
func Fee(w rest.ResponseWriter, r *rest.Request) {
	delegaterStr := r.PathParam("delegater")
	delegater := common.HexToAddress(delegaterStr)
	a := db.AccountGetAccount(delegater)
	res := &feeReponse{
		Available: models.AccountAvailable(a),
		NeedSmt:   a.NeedSmt,
	}
	err := w.WriteJson(res)
	if err != nil {

	}
}

type feeReponse struct {
	Available *big.Int
	NeedSmt   *big.Int
}
