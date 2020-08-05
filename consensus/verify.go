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
