package cvm

import (
	"fmt"

	"github.com/wupeaking/pbft_impl/common/config"
	"github.com/wupeaking/pbft_impl/model"
	"github.com/wupeaking/pbft_impl/storage/cache"
)

// 交易执行虚拟机

type VirtualMachine struct {
	db *cache.DBCache
}

func New(db *cache.DBCache, cfg *config.Configure) *VirtualMachine {
	return &VirtualMachine{
		db: db,
	}
}

// 模拟交易
func (vm *VirtualMachine) Eval(tx *model.Tx) *model.TxReceipt {
	txr := &model.TxReceipt{}
	txr.Status = -1
	txr.TxId = tx.Sign
	if tx.Sender == nil || tx.Sender.Address == "" ||
		tx.Sequeue == "" || len(tx.Sign) == 0 || len(tx.PublickKey) == 0 {
		return txr
	}

	// 查询当前交易是否已经存在
	old, err := vm.db.GetTxByID(fmt.Sprintf("%0x", tx.Sign))
	if err != nil || old != nil {
		return txr
	}

	// 首先交易 签名是否正确
	accountAddr := model.PublicKeyToAddress(tx.PublickKey)
	if tx.Sender.Address != accountAddr.Address {
		return txr
	}
	// 查询账户信息
	account, err := vm.db.GetAccountByID(tx.Sender.Address)
	if err != nil {
		return txr
	}
	if model.Compare(account.Balance.Amount, tx.Amount.Amount) < 0 {
		return txr
	}

	// 签名
	ok, err := tx.VerifySignedTx()
	if err != nil || !ok {
		return txr
	}

	txr.Status = 0
	txr.TxId = tx.Sign
	return txr
}

// 执行交易
func (vm *VirtualMachine) Exec(tx *model.Tx) (*model.TxReceipt, error) {
	// uid := uuid.NewV4()
	txr := &model.TxReceipt{}
	txr.Status = -1
	// txr.Sequeue = strings.Replace(uid.String(), "-", "", -1)
	txr.TxId = tx.Sign
	if tx.Sender == nil || tx.Sender.Address == "" ||
		tx.Sequeue == "" || len(tx.Sign) == 0 || len(tx.PublickKey) == 0 {
		return txr, fmt.Errorf("交易参数不正确")
	}
	// 查询当前交易是否已经存在
	old, err := vm.db.GetTxByID(fmt.Sprintf("%0x", tx.Sign))
	if err != nil || old != nil {
		return txr, fmt.Errorf("交易已经存在")
	}

	// 首先交易 签名是否正确
	accountAddr := model.PublicKeyToAddress(tx.PublickKey)
	if tx.Sender.Address != accountAddr.Address {
		return txr, fmt.Errorf("公钥和地址不匹配")
	}
	// 查询账户信息
	account, err := vm.db.GetAccountByID(tx.Sender.Address)
	if err != nil {
		return txr, fmt.Errorf("查询账户信息出错 err: %v", err)
	}
	if model.Compare(account.Balance.Amount, tx.Amount.Amount) < 0 {
		return txr, fmt.Errorf("账户余额不足")
	}

	// 签名
	ok, err := tx.VerifySignedTx()
	if err != nil || !ok {
		return txr, fmt.Errorf("交易签名不一致")
	}

	// 获取receipt的账户
	recv, err := vm.db.GetAccountByID(tx.Recipient.Address)
	if err != nil {
		return txr, err
	}
	if recv == nil {
		// 说明账户不存在 需要新建一个账户
		recv = &model.Account{
			Id:          tx.Recipient,
			Balance:     &model.Amount{Amount: "0"},
			AccountType: int32(model.AccountType_Normal),
		}
	}
	recv.Balance.AddAmount(tx.Amount)
	account.Balance.SubAmount(tx.Amount)
	txr.Status = 0

	err = vm.db.Insert(account)
	if err != nil {
		return txr, err
	}

	err = vm.db.Insert(recv)
	if err != nil {
		return txr, err
	}
	return txr, nil
}
