package model

import "fmt"

func (msg *PbftMessage) setValue(val interface{}) error {
	switch x := val.(type) {
	case *PbftGenericMessage:
		msg.Msg = &PbftMessage_Generic{Generic: x}
	case *PbftViewChange:
		msg.Msg = &PbftMessage_ViewChange{ViewChange: x}
	default:
		return fmt.Errorf("PbftMessage.Value has unexpected type %T", x)
	}
	return nil
}

func NewPbftMessage(val interface{}) *PbftMessage {
	msg := &PbftMessage{}
	msg.setValue(val)
	return msg
}
