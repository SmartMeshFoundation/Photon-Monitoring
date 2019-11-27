package models

import (
	"fmt"
	"math/big"
	"time"

	"github.com/SmartMeshFoundation/Photon/log"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
)

/*
ReceiveDelegate 接受用户委托核心方法, 暂时放在models包是为了方便,逻辑上应独立
*/
func (model *ModelDB) ReceiveDelegate(c *ChannelFor3rd, delegator common.Address) (err error) {
	lastBlockNumber := model.GetLatestBlockNumber()
	// 0. 开启事务
	tx := model.db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	delegateKey := BuildDelegateKey(c.ChannelIdentifier, delegator)
	// 1. 追加Punish部分
	newSMT4Punish, err := appendDelegatePunish(tx, delegateKey, c.Punishes)
	if err != nil {
		return
	}

	// 2. 追加AnnounceDispose部分
	err = appendDelegateAnnounceDispose(tx, delegateKey, c.AnnouceDisposed)
	if err != nil {
		return
	}

	// 3. 更新Delegate
	d, oldNeedSMT, err := updateDelegate(tx, c, delegator, lastBlockNumber, newSMT4Punish)
	if err != nil {
		return
	}

	// 4. 更新收费信息
	updateAccountSMT(tx, delegator, oldNeedSMT, d.NeedSMT())
	return
}

func updateDelegate(tx *gorm.DB, c *ChannelFor3rd, delegator common.Address, lastBlockNumber int64, newSMT4Punish *big.Int) (d *Delegate, oldNeedSMT *big.Int, err error) {
	isFirst := false
	// 1. 获取delegate对象
	d = &Delegate{
		Key: BuildDelegateKey(c.ChannelIdentifier, delegator),
	}
	err = tx.Where(d).Find(d).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
		isFirst = true
	}
	if err != nil {
		err = fmt.Errorf("err when find Delegate from db : %s", err.Error())
		return
	}
	// 2. 更新
	if isFirst {
		oldNeedSMT = big.NewInt(0)
		// 保存全量信息到数据库
		d.ChannelIdentifierStr = c.ChannelIdentifier.String()
		d.OpenBlockNumber = c.OpenBlockNumber
		d.TokenAddressStr = c.TokenAddress.String()
		d.DelegatorAddressStr = delegator.String()
		d.PartnerAddressStr = c.PartnerAddress.String()
		d.SettleBlockNumber = c.settleBlockNumber
		d.DelegateTimestamp = time.Now().Unix()
		d.DelegateBlockNumber = lastBlockNumber
		d.Status = DelegateStatusInit
		d.Error = ""
		d.SetUpdateBalanceProof(c.GetDelegateUpdateBalanceProof())
		d.SetUnlocks(c.GetDelegateUnlocks())
		// 如果是close之后的第一次委托,注册监听
		if c.settleBlockNumber > 0 {
			AddDelegateMonitorInTx(tx, d)
		}
	} else {
		oldNeedSMT = d.NeedSMT()
		// 非第一次委托,仅更新部分数据,并校验部分信息(非测试环境)

		// OpenBlockNumber不匹配
		// 当新值<旧值时,不可能出现的情况,直接报错回滚
		// 当新值>旧值时,可能是用户在进行了取现/重开,但是我监听了事件的,应该已经删除了,所以也不应该出现,同样报错回滚
		if !params.DebugMode && c.OpenBlockNumber != d.OpenBlockNumber {
			err = fmt.Errorf("excepition because new OpenBlockNumber is not equal, old=%d new=%d ", d.OpenBlockNumber, c.OpenBlockNumber)
			return
		}

		// 状态校验,如果委托状态已经不为init了,则不允许更新UpdateBalanceProof及unlocks,但是不报错,不影响Punishes及AnnounceDispose的更新
		if d.Status == DelegateStatusInit {
			// nonce校验
			if !params.DebugMode && d.UpdateBalanceProof().Nonce > c.UpdateTransfer.Nonce {
				err = fmt.Errorf("only delegate newer nonce ,old nonce=%d,new=%d", d.UpdateBalanceProof().Nonce, c.UpdateTransfer.Nonce)
				return
			}
			d.SetUpdateBalanceProof(c.GetDelegateUpdateBalanceProof())
			d.SetUnlocks(c.GetDelegateUnlocks())
		}
		// 这里不用更新SettleBlockNumber及注册监听,如果是第一次委托就已经close,上面if里面会做这部分工作
		// 如果之前已经委托过,那么会在收到通道关闭事件的时候做这部分工作
		d.DelegateTimestamp = time.Now().Unix()
		d.DelegateBlockNumber = lastBlockNumber
	}
	// 2.5 全量更新Secret
	d.SetSecrets(c.GetDeleteSecrets())

	// 3. 更新计费信息
	d.CalcNeedSMT(newSMT4Punish)
	// 4. 更新
	err = tx.Save(d).Error
	if err != nil {
		err = fmt.Errorf("db err when update Delegate : %s", err.Error())
	}
	return
}

/*
无关状态,直接追加,最多就是追加的数据无效了
*/
func appendDelegatePunish(tx *gorm.DB, delegateKey []byte, newPunishes []*Punish) (newSMT4Punish *big.Int, err error) {
	newSMT4Punish = big.NewInt(0)
	// 1. 查询已经存在的委托
	var all []*DelegatePunish
	err = tx.Where(&DelegatePunish{
		DelegateKey: delegateKey,
	}).Find(&all).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		err = fmt.Errorf("db err when find DelegatePunish : %s", err.Error())
		return
	}
	allMap := make(map[string]*DelegatePunish)
	for _, old := range all {
		allMap[old.LockHashStr] = old
	}
	// 2. 添加新的委托
	for _, punish := range newPunishes {
		if _, exist := allMap[punish.LockHash.String()]; exist {
			continue
		}
		err = tx.Save(&DelegatePunish{
			LockHashStr:       punish.LockHash.String(),
			DelegateKey:       delegateKey,
			AdditionalHashStr: punish.AdditionalHash.String(),
			Signature:         punish.Signature,
		}).Error
		if err != nil {
			err = fmt.Errorf("db err when add DelegatePunish : %s", err.Error())
			return
		}
	}
	// 3. 计算花费,不管旧的新的,只要存在punish委托,就计算费用,且只计算一次
	if len(all) > 0 || len(newPunishes) > 0 {
		newSMT4Punish = newSMT4Punish.Add(newSMT4Punish, params.SmtPunish)
	}
	return
}

/*
无关状态,直接追加,最多就是追加的数据无效了
*/
func appendDelegateAnnounceDispose(tx *gorm.DB, delegateKey []byte, newAnnounceDisposes []*AnnouceDisposed) (err error) {
	if len(newAnnounceDisposes) <= 0 {
		return
	}
	// 1. 查询已经存在的委托
	//todo 可以不用查询委托是否存在,直接通过数据库的主键冲突来检测这种错误,避免加载所有的DelegateAnnounceDispose,
	//然后后工比较
	var all []*DelegateAnnounceDispose
	err = tx.Where(&DelegateAnnounceDispose{
		DelegateKey: delegateKey,
	}).Find(&all).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		err = fmt.Errorf("db err when find DelegateAnnounceDispose : %s", err.Error())
		return
	}
	allMap := make(map[string]*DelegateAnnounceDispose)
	for _, old := range all {
		allMap[old.LockSecretHashStr] = old
	}
	// 2. 添加新的委托
	for _, ad := range newAnnounceDisposes {
		if _, exist := allMap[ad.LockSecretHash.String()]; exist {
			continue
		}
		err = tx.Save(&DelegateAnnounceDispose{
			LockSecretHashStr: ad.LockSecretHash.String(),
			DelegateKey:       delegateKey,
		}).Error
		if err != nil {
			err = fmt.Errorf("db err when add DelegateAnnounceDispose : %s", err.Error())
			return
		}
	}
	return
}

func updateAccountSMT(tx *gorm.DB, delegator common.Address, oldSmt *big.Int, newSmt *big.Int) {
	if oldSmt.Cmp(newSmt) == 0 {
		// 无需更新
		return
	}
	a := GetAccountInTx(tx, delegator)
	a.NeedSmt.Add(a.NeedSmt, newSmt)
	a.NeedSmt.Sub(a.NeedSmt, oldSmt)
	log.Trace(fmt.Sprintf("account=%s newsmt=%s,oldsmt=%s", a, newSmt, oldSmt))
	err := tx.Save(a.toSerialization()).Error
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
}
