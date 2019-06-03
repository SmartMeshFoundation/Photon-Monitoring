package restful

import (
	"fmt"

	"github.com/SmartMeshFoundation/Photon/dto"

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
	var err error
	delegaterStr := r.PathParam("delegater")
	channel := r.PathParam("channel")
	delegater := common.HexToAddress(delegaterStr)
	if delegater == utils.EmptyAddress || channel == utils.EmptyAddress.String() {
		err = w.WriteJson(&dto.APIResponse{
			ErrorCode: delegateError,
			ErrorMsg:  "arg error",
		})
		return
	}
	channelIdentifier := common.HexToHash(channel)
	delegateKey := models.BuildDelegateKey(channelIdentifier, delegater)
	// delegate
	d, err := db.GetDelegateByKey(delegateKey)
	if err != nil {
		err = w.WriteJson(&dto.APIResponse{
			ErrorCode: delegateError,
			ErrorMsg:  fmt.Sprintf("db GetDelegateByKey err : %s", err.Error()),
		})
		return
	}
	// DelegatePunishList
	dps, err := db.GetDelegatePunishListByDelegateKey(delegateKey)
	if err != nil {
		err = w.WriteJson(&dto.APIResponse{
			ErrorCode: delegateError,
			ErrorMsg:  fmt.Sprintf("db GetDelegateUpdateBalanceProofByDelegateKey err : %s", err.Error()),
		})
		return
	}
	// DelegateAnnounceDisposeList
	das, err := db.GetDelegateAnnounceDisposeListByDelegateKey(delegateKey)
	if err != nil {
		err = w.WriteJson(&dto.APIResponse{
			ErrorCode: delegateError,
			ErrorMsg:  fmt.Sprintf("db GetDelegateUpdateBalanceProofByDelegateKey err : %s", err.Error()),
		})
		return
	}
	res := dto.NewAPIResponse(nil, &delegateInfoResponse{
		Delegate:                   d,
		DelegateUpdateBalanceProof: d.UpdateBalanceProof(),
		DelegateUnlocks:            d.Unlocks(),
		DelegatePunishes:           dps,
		DelegateAnnounceDispose:    das,
	})

	if db.AccountIsBalanceEnough(delegater) {
		res.ErrorCode = delegateSuccess
	} else {
		res.ErrorCode = delegateSuccessButNotEnoughBalance
	}

	err = w.WriteJson(res)
	if err != nil {
		log.Error(fmt.Sprintf("write json err %s", err))
	}
}

type delegateInfoResponse struct {
	Delegate                   *models.Delegate                   `json:"delegate"`
	DelegateUpdateBalanceProof *models.DelegateUpdateBalanceProof `json:"delegate_update_balance_proof"`
	DelegateUnlocks            []*models.DelegateUnlock           `json:"delegate_unlocks"`
	DelegatePunishes           []*models.DelegatePunish           `json:"delegate_punishes"`
	DelegateAnnounceDispose    []*models.DelegateAnnounceDispose  `json:"delegate_announce_dispose"`
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
