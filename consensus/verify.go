package consensus

import (
	"crypto/sha256"
	"fmt"

	"github.com/golang/protobuf/proto"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func (pbft *PBFT) VerfifyMsg(msg *model.PbftMessage) bool {
	if !pbft.isValidMsg(msg) {
		return false
	}
	if gm := msg.GetGeneric(); gm != nil {
		if !pbft.verfifyMsgInfo(gm.Info) {
			return false
		}
		for i := range gm.OtherInfos {
			if !pbft.verfifyMsgInfo(gm.OtherInfos[i]) {
				return false
			}
		}
		if gm.Block != nil {
			if !pbft.verfifyBlock(gm.Block) {
				return false
			}
		}
		return true
	}

	if vc := msg.GetViewChange(); vc != nil {
		if !pbft.verfifyMsgInfo(vc.Info) {
			return false
		}
		return true
	}
	return false
}

func (pbft *PBFT) verfifyMsgInfo(msgInfo *model.PbftMessageInfo) bool {
	pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("%0x", msgInfo.SignerId))
	if err != nil {
		return false
	}

	info := model.PbftMessageInfo{
		MsgType: msgInfo.MsgType,
		View:    msgInfo.View,
		SeqNum:  msgInfo.SeqNum,
	}
	content, _ := proto.Marshal(&info)
	hash := sha256.New().Sum(content)
	return cryptogo.VerifySign(pubKey, fmt.Sprintf("%0x", msgInfo.Sign), fmt.Sprintf("%0x", hash))
}

func (pbft *PBFT) verfifyBlock(blk *model.PbftBlock) bool {
	pubKey, err := cryptogo.LoadPublicKey(fmt.Sprintf("%0x", blk.SignerId))
	if err != nil {
		return false
	}

	b := model.PbftBlock{
		BlockId:  blk.BlockId,
		BlockNum: blk.BlockNum,
		Content:  blk.Content,
	}
	content, _ := proto.Marshal(&b)
	hash := sha256.New().Sum(content)
	return cryptogo.VerifySign(pubKey, fmt.Sprintf("%0x", blk.Sign), fmt.Sprintf("%0x", hash))
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
	return nil, fmt.Errorf("为支持的消息类型")
}

func (pbft *PBFT) signMsgInfo(msgInfo *model.PbftMessageInfo) (*model.PbftMessageInfo, error) {
	privKey, err := cryptogo.LoadPrivateKey(fmt.Sprintf("%0x", pbft.ws.CurVerfier.PrivateKey))
	if err != nil {
		return nil, err
	}
	info := model.PbftMessageInfo{
		MsgType: msgInfo.MsgType,
		View:    msgInfo.View,
		SeqNum:  msgInfo.SeqNum,
	}
	content, _ := proto.Marshal(&info)
	hash := sha256.New().Sum(content)
	sign, err := cryptogo.Sign(privKey, hash)
	if err != nil {
		return nil, err
	}
	s, err := cryptogo.Hex2Bytes(sign)
	if err != nil {
		return nil, err
	}
	msgInfo.Sign = s
	return msgInfo, nil
}

func (pbft *PBFT) signBlock(blk *model.PbftBlock) (*model.PbftBlock, error) {
	privKey, err := cryptogo.LoadPrivateKey(fmt.Sprintf("%0x", pbft.ws.CurVerfier.PrivateKey))
	if err != nil {
		return nil, err
	}
	b := model.PbftBlock{
		BlockId:  blk.BlockId,
		BlockNum: blk.BlockNum,
		Content:  blk.Content,
	}
	content, _ := proto.Marshal(&b)
	hash := sha256.New().Sum(content)
	sign, err := cryptogo.Sign(privKey, hash)
	if err != nil {
		return nil, err
	}
	s, err := cryptogo.Hex2Bytes(sign)
	if err != nil {
		return nil, err
	}
	blk.Sign = s
	return blk, nil
}
