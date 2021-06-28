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
func (vm *VirtualMachine) Eval(tx *model.Tx, snap *Snapshot) (*model.TxReceipt, error) {
	txr := &model.TxReceipt{}
	txr.Status = -1
	txr.TxId = tx.Sign
	if tx.Sender == nil || tx.Sender.Address == "" ||
		tx.Sequeue == "" || len(tx.Sign) == 0 || len(tx.PublickKey) == 0 {
		return txr, fmt.Errorf("交易参数不正确")
	}

	// 查询当前交易是否已经存在
	txID := fmt.Sprintf("%0x", tx.Sign)
	if snap.GetTxByID(txID) != nil {
		return txr, fmt.Errorf("交易已经存在")
	}
	old, err := vm.db.GetTxByID(txID)
	if err != nil || old != nil {
		return txr, fmt.Errorf("交易已经存在 err: %v", err)
	}

	// 首先交易 签名是否正确
	accountAddr := model.PublicKeyToAddress(tx.PublickKey)
	if tx.Sender.Address != accountAddr.Address {
		return txr, fmt.Errorf("公钥和地址不匹配")
	}
	// 查询账户信息
	var account *model.Account
	if a := snap.GetAccountByID(tx.Sender.Address); a != nil {
		if model.Compare(account.Balance.Amount, tx.Amount.Amount) < 0 {
			return txr, nil
		}
		account = a
	} else {
		a, err := vm.db.GetAccountByID(tx.Sender.Address)
		if err != nil {
			return txr, err
		}
		if a == nil {
			return txr, fmt.Errorf("账户不存在")
		}
		if model.Compare(a.Balance.Amount, tx.Amount.Amount) < 0 {
			return txr, nil
		}
		account = a
	}

	// 签名
	ok, err := tx.VerifySignedTx()
	if err != nil || !ok {
		return txr, fmt.Errorf("交易签名不一致")
	}

	// 获取receipt的账户
	var recv *model.Account
	if r := snap.GetAccountByID(tx.Recipient.Address); r != nil {
		recv = r
	} else {
		r, err := vm.db.GetAccountByID(tx.Recipient.Address)
		if err != nil {
			return txr, err
		}
		if r == nil {
			// 说明账户不存在 需要新建一个账户
			recv = &model.Account{
				Id:          tx.Recipient,
				Balance:     &model.Amount{Amount: "0"},
				AccountType: int32(model.AccountType_Normal),
			}
		} else {
			recv = r
		}
	}

	// 复制一份账户 因为整个过程都是指针流转 会导致账户状态发送变化
	recvCopy := vm.CopyAccount(recv)
	accountCopy := vm.CopyAccount(account)
	recvCopy.Balance.AddAmount(tx.Amount)
	accountCopy.Balance.SubAmount(tx.Amount)
	txr.Status = 0
	txr.TxId = tx.Sign
	snap.UpdateAccountByID(accountCopy)
	snap.UpdateAccountByID(recvCopy)
	snap.UpdateTxByID(tx)
	return txr, nil
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
		return txr, fmt.Errorf("交易已经存在 err: %v", err)
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
	if account == nil {
		return txr, fmt.Errorf("账户不存在")
	}
	if model.Compare(account.Balance.Amount, tx.Amount.Amount) < 0 {
		return txr, nil //fmt.Errorf("账户余额不足")
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

	// 插入交易
	err = vm.db.Insert(tx)
	println("insert tx: ", fmt.Sprintf("%0x", tx.Sign))
	if err != nil {
		return txr, err
	}
	// 插入交易收据
	err = vm.db.Insert(txr)
	if err != nil {
		return txr, err
	}
	return txr, nil
}

func (vm *VirtualMachine) CopyAccount(account *model.Account) *model.Account {
	code := make([]byte, 0, len(account.Code))
	copy(code, account.Code)
	return &model.Account{
		Id:          account.Id,
		Code:        code,
		Balance:     &model.Amount{Amount: account.Balance.Amount},
		AccountType: account.AccountType,
		PublickKey:  account.PublickKey,
	}
}
