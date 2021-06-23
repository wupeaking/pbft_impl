package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/golang/protobuf/proto"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) VerfifyMsg(msg *model.PbftMessage) bool {
	if !pbft.isValidMsg(msg) {
		pbft.logger.Debugf("消息格式错误")
		return false
	}
	if gm := msg.GetGeneric(); gm != nil {
		if !pbft.verfifyMsgInfo(gm.Info) {
			pbft.logger.Debugf("通用消息验证失败")
			return false
		}
		for i := range gm.OtherInfos {
			if !pbft.verfifyMsgInfo(gm.OtherInfos[i]) {
				pbft.logger.Debugf("其他通用消息验证失败")
				return false
			}
		}
		if gm.Block != nil {
			if !pbft.verfifyBlock(gm.Block) {
				pbft.logger.Debugf("消息内的区块验证失败")
				return false
			}
		}
		return true
	}

	if vc := msg.GetViewChange(); vc != nil {
		if !pbft.verfifyMsgInfo(vc.Info) {
			pbft.logger.Debugf("视图消息验证失败")
			return false
		}
		return true
	}
	return false
}

func (pbft *PBFT) GetMsgSigner(msg *model.PbftMessage) []byte {
	if !pbft.isValidMsg(msg) {
		return nil
	}
	if gm := msg.GetGeneric(); gm != nil {
		return gm.Info.SignerId
	}

	if vc := msg.GetViewChange(); vc != nil {
		return vc.Info.SignerId
	}
	return nil
}

func (pbft *PBFT) verfifyMsgInfo(msgInfo *model.PbftMessageInfo) bool {
	// 2021年06月10日11:08:11 取消掉单独的协调者角色 由验证节点自己充当协调着
	// if msgInfo.MsgType == model.MessageType_NewBlockProposal {
	// 	return pbft.verfifyBlockProposalMsg(msgInfo)
	// }
	if !pbft.IsVaildVerifier(msgInfo.SignerId) {
		return false
	}

	pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("0x%x", msgInfo.SignerId))
	if err != nil {
		return false
	}

	info := model.PbftMessageInfo{
		MsgType: msgInfo.MsgType,
		View:    msgInfo.View,
		SeqNum:  msgInfo.SeqNum,
	}
	content, _ := proto.Marshal(&info)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	return cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", msgInfo.Sign), fmt.Sprintf("0x%x", hash))
}

func (pbft *PBFT) verfifyBlockProposalMsg(msgInfo *model.PbftMessageInfo) bool {
	pub, _ := cryptogo.Hex2Bytes(pbft.cfg.Coordinator.Publickey)
	if bytes.Compare(pub, msgInfo.SignerId) != 0 {
		return false
	}
	pubKey, err := cryptogo.LoadPublicKey(pbft.cfg.Coordinator.Publickey)
	if err != nil {
		return false
	}
	info := model.PbftMessageInfo{
		MsgType: msgInfo.MsgType,
		View:    msgInfo.View,
		SeqNum:  msgInfo.SeqNum,
	}
	content, _ := proto.Marshal(&info)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	return cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", msgInfo.Sign), fmt.Sprintf("0x%x", hash))
}

// verfifyBlock 校验主签名者的签名
func (pbft *PBFT) verfifyBlock(blk *model.PbftBlock) bool {
	if !pbft.IsVaildVerifier(blk.SignerId) {
		pbft.logger.Debugf("不是有效的验证者 no: %d", blk.BlockNum)
		return false
	}

	pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("0x%x", blk.SignerId))
	if err != nil {
		pbft.logger.Debugf("公钥加载失败, err:%v no: %d", err, blk.BlockNum)
		return false
	}

	// 验证交易执行是否正确
	if err := pbft.TryApplyBlock(blk); err != nil {
		pbft.logger.Warnf("区块交易执行失败 err: %s", err.Error())
		return false
	}

	b := model.PbftBlock{
		PrevBlock:      blk.PrevBlock,
		BlockNum:       blk.BlockNum,
		TxRoot:         blk.TxRoot,
		TxReceiptsRoot: blk.TxReceiptsRoot,
		TimeStamp:      blk.TimeStamp,
		BlockId:        "",
		View:           blk.View,
	}

	content, _ := proto.Marshal(&b)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	if blk.BlockId != hex.EncodeToString(hash) {
		pbft.logger.Debugf("block hash 校验不一致")
		return false
	}
	return cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", blk.Sign), fmt.Sprintf("0x%x", hash))
}

// VerfifyMostBlock 验证有超过2/3的节点已对区块进行了签名
func (pbft *PBFT) VerfifyMostBlock(blk *model.PbftBlock) bool {
	if blk.BlockNum == 0 {
		return pbft.VerfifyGenesisBlock(blk)
	}

	if !pbft.IsVaildVerifier(blk.SignerId) {
		return false
	}
	// 验证交易执行是否正确
	if err := pbft.TryApplyBlock(blk); err != nil {
		pbft.logger.Warnf("区块交易执行失败 err: %s", err.Error())
		return false
	}

	b := model.PbftBlock{
		PrevBlock:      blk.PrevBlock,
		BlockNum:       blk.BlockNum,
		TxRoot:         blk.TxRoot,
		TxReceiptsRoot: blk.TxReceiptsRoot,
		TimeStamp:      blk.TimeStamp,
		BlockId:        "",
		View:           blk.View,
	}

	content, _ := proto.Marshal(&b)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	if blk.BlockId != hex.EncodeToString(hash) {
		return false
	}
	pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("0x%x", blk.SignerId))
	if err != nil {
		return false
	}
	if !cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", blk.Sign), fmt.Sprintf("0x%x", hash)) {
		return false
	}

	f := len(pbft.ws.Verifiers) / 3
	var minNodes int
	if f == 0 {
		minNodes = len(pbft.ws.Verifiers)
	} else {
		minNodes = 2*f + 1
	}

	cnt := 0
	for _, pair := range blk.SignPairs {
		if !pbft.IsVaildVerifier(pair.SignerId) {
			continue
		}

		pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("0x%x", pair.SignerId))
		if err != nil {
			return false
		}
		if cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", pair.Sign), fmt.Sprintf("0x%x", hash)) {
			cnt++
		}
	}
	if cnt+1 >= minNodes {
		return true
	}
	return false
}

// VerfifyBlockHeader 验证区块头　需要超过2/3f才能成功
func (pbft *PBFT) VerfifyBlockHeader(blk *model.PbftBlock) bool {
	if blk.BlockNum == 0 {
		return pbft.VerfifyGenesisBlock(blk)
	}

	if !pbft.IsVaildVerifier(blk.SignerId) {
		pbft.logger.Debugf("不是有效的验证者")
		return false
	}

	hash, _ := cryptogo.Hex2Bytes(blk.BlockId)
	pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("0x%x", blk.SignerId))
	if err != nil {
		pbft.logger.Debugf("加载公钥失败 err:%v", err)
		return false
	}
	if !cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", blk.Sign), fmt.Sprintf("0x%x", hash)) {
		pbft.logger.Debugf("验证签名失败 ")
		return false
	}

	f := len(pbft.ws.Verifiers) / 3
	var minNodes int
	if f == 0 {
		minNodes = len(pbft.ws.Verifiers)
	} else {
		minNodes = 2*f + 1
	}

	cnt := 0
	for _, pair := range blk.SignPairs {
		if !pbft.IsVaildVerifier(pair.SignerId) {
			continue
		}

		pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("0x%x", pair.SignerId))
		if err != nil {
			return false
		}
		if cryptogo.VerifySign(pubKey, fmt.Sprintf("0x%x", pair.Sign), fmt.Sprintf("0x%x", hash)) {
			cnt++
		}
	}
	if cnt+1 >= minNodes {
		return true
	}
	return false
}

func (pbft *PBFT) SignMsg(msg *model.PbftMessage) (*model.PbftMessage, error) {
	if !pbft.isValidMsg(msg) {
		return nil, fmt.Errorf("msg is nil")
	}
	if gm := msg.GetGeneric(); gm != nil {
		info, err := pbft.signMsgInfo(gm.Info)
		if err != nil {
			return nil, err
		}
		gm.Info = info

		for i := range gm.OtherInfos {
			other, err := pbft.signMsgInfo(gm.OtherInfos[i])
			if err != nil {
				return nil, err
			}
			gm.OtherInfos[i] = other
		}

		if gm.Block != nil {
			blk, err := pbft.signBlock(gm.Block)
			if err != nil {
				return nil, err
			}
			gm.Block = blk
		}
		return model.NewPbftMessage(gm), nil
	}

	if vc := msg.GetViewChange(); vc != nil {
		info, err := pbft.signMsgInfo(vc.Info)
		if err != nil {
			return nil, err
		}
		vc.Info = info
		return model.NewPbftMessage(vc), nil
	}
	return nil, fmt.Errorf("未支持的消息类型")
}

func (pbft *PBFT) signMsgInfo(msgInfo *model.PbftMessageInfo) (*model.PbftMessageInfo, error) {
	privKey, err := cryptogo.LoadPrivateKey(fmt.Sprintf("0x%x", pbft.ws.CurVerfier.PrivateKey))
	if err != nil {
		return nil, err
	}
	info := model.PbftMessageInfo{
		MsgType: msgInfo.MsgType,
		View:    msgInfo.View,
		SeqNum:  msgInfo.SeqNum,
	}
	content, _ := proto.Marshal(&info)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	sign, err := cryptogo.Sign(privKey, hash)
	if err != nil {
		return nil, err
	}
	s, err := cryptogo.Hex2Bytes(sign)
	if err != nil {
		return nil, err
	}
	msgInfo.Sign = s
	msgInfo.SignerId = pbft.ws.CurVerfier.PublickKey
	return msgInfo, nil
}

// signBlock 对区块进行签名 如果是主节点进行主签名 否则进行次签名 如果已近签名过则不签名
func (pbft *PBFT) signBlock(blk *model.PbftBlock) (*model.PbftBlock, error) {
	privKey, err := cryptogo.LoadPrivateKey(fmt.Sprintf("0x%x", pbft.ws.CurVerfier.PrivateKey))
	if err != nil {
		return nil, err
	}
	b := model.PbftBlock{
		PrevBlock:      blk.PrevBlock,
		BlockNum:       blk.BlockNum,
		TxRoot:         blk.TxRoot,
		TxReceiptsRoot: blk.TxReceiptsRoot,
		TimeStamp:      blk.TimeStamp,
		BlockId:        "",
		View:           blk.View,
	}
	content, _ := proto.Marshal(&b)
	sh := sha256.New()
	sh.Write(content)
	hash := sh.Sum(nil)
	sign, err := cryptogo.Sign(privKey, hash)
	if err != nil {
		return nil, err
	}
	s, err := cryptogo.Hex2Bytes(sign)
	if err != nil {
		return nil, err
	}
	blk.BlockId = hex.EncodeToString(hash)

	if pbft.IsPrimaryVerfier() {
		blk.SignerId = pbft.ws.CurVerfier.PublickKey
		blk.Sign = s
	} else {
		if blk.SignPairs == nil {
			blk.SignPairs = make([]*model.SignPairs, 0)
		}

		for i := range blk.SignPairs {
			if bytes.Compare(blk.SignPairs[i].SignerId, pbft.ws.CurVerfier.PublickKey) == 0 {
				return blk, nil
			}
		}
		blk.SignPairs = append(blk.SignPairs, &model.SignPairs{
			SignerId: pbft.ws.CurVerfier.PublickKey,
			Sign:     s,
		})
	}

	return blk, nil
}

func (pbft *PBFT) IsVaildVerifier(singerID []byte) bool {
	return pbft.ws.IsVerfier(singerID)
}

func (pbft *PBFT) isValidMsg(msg *model.PbftMessage) bool {
	if msg == nil {
		return false
	}
	gm := msg.GetGeneric()
	if gm != nil {
		if gm.Info == nil {
			return false
		}
		return true
	}

	vc := msg.GetViewChange()
	if vc != nil {
		if vc.Info == nil {
			return false
		}
		return true
	}
	return false
}

func (pbft *PBFT) IsPrimaryVerfier() bool {
	primary := (pbft.ws.BlockNum + 1 + pbft.ws.View) % uint64(len(pbft.ws.Verifiers))
	if len(pbft.ws.Verifiers) == 1 {
		primary = 0
	}
	return bytes.Compare(pbft.ws.Verifiers[primary].PublickKey, pbft.ws.CurVerfier.PublickKey) == 0
}

func (pbft *PBFT) VerfifyGenesisBlock(blk *model.PbftBlock) bool {
	g, err := pbft.ws.GetBlock(0)
	if err != nil {
		pbft.logger.Errorf("获取本地创世区块错误")
	}
	if g.BlockId != blk.BlockId {
		return false
	}
	if g.BlockNum != blk.BlockNum {
		return false
	}
	if g.TimeStamp != blk.TimeStamp {
		return false
	}
	if len(g.SignPairs) != len(blk.SignPairs) {
		return false
	}
	for i := range g.SignPairs {
		if bytes.Compare(g.SignPairs[i].SignerId, blk.SignPairs[i].SignerId) != 0 {
			return false
		}
	}
	return true
}
