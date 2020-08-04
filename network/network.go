package network

type SwitcherI interface {
	Broadcast(msg interface{}) error
	Recv() <-chan interface{}
}
