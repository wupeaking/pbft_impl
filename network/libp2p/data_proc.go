package libp2p

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/libp2p/go-libp2p-core/network"
	nt "github.com/wupeaking/pbft_impl/network"
)

func (p2p *P2PNetWork) dataStreamRecv(stream network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	for {
		// todo:: 此处需要修改
		// 读取数据流
		// 解码数据
		// 反序列化结构
		// 回调对应的消息模块
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			break
		}

		if str == "" {
			break
		}
		if str != "\n" {
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}
	}
	stream.Conn().Close()
	p2p.Lock()
	delete(p2p.books, stream.ID())
	p2p.Unlock()
}

func (p2p *P2PNetWork) packageData(msg *nt.BroadcastMsg) ([]byte, error) {
	dataBuf := bytes.NewBuffer(nil)
	// 加入magic 头
	dataBuf.WriteByte(0x89)
	dataBuf.WriteByte(0x08)
	dataBuf.WriteByte(0x05)
	dataBuf.WriteByte(0x20)
	dataBuf.WriteByte(0x20)

	msgBuf, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	lenBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(lenBuf, uint64(len(msgBuf)))
	// binary.PutUvarint(lenBuf, uint64(len(msgBuf)))
	dataBuf.Write(lenBuf)
	dataBuf.Write(msgBuf)

	needCK := dataBuf.Bytes()
	ck := crc32.ChecksumIEEE(needCK)
	ckBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(ckBuf, ck)
	dataBuf.Reset()

	dataBuf.Write(needCK)
	dataBuf.Write(ckBuf)

	return dataBuf.Bytes(), nil
}

func (p2p *P2PNetWork) unpackageData(rw *bufio.ReadWriter) (*nt.BroadcastMsg, error) {
	// 尝试读取magic 头
	magicHeader := make([]byte, 5)
	_, err := io.ReadFull(rw, magicHeader)
	// magicHeader, err := rw.Peek(5)
	if err != nil {
		return nil, err
	}
	if bytes.Compare(magicHeader, []byte{0x89, 0x08, 0x05, 0x20, 0x20}) != 0 {
		return nil, fmt.Errorf("magic header err %v", magicHeader)
	}
	msgLenBuf := make([]byte, 8)
	_, err = io.ReadFull(rw, msgLenBuf)
	if err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint64(msgLenBuf) //(lenBuf, uint64(len(msgBuf)))
	msgBuf := make([]byte, msgLen)
	_, err = io.ReadFull(rw, msgBuf)
	if err != nil {
		return nil, err
	}
	// 读取ck
	checkBuf := make([]byte, 4)
	_, err = io.ReadFull(rw, checkBuf)
	if err != nil {
		return nil, err
	}
	readCk := binary.BigEndian.Uint32(checkBuf)

	dataBuf := bytes.NewBuffer(nil)
	// 加入magic 头
	dataBuf.WriteByte(0x89)
	dataBuf.WriteByte(0x08)
	dataBuf.WriteByte(0x05)
	dataBuf.WriteByte(0x20)
	dataBuf.WriteByte(0x20)
	dataBuf.Write(msgLenBuf)
	dataBuf.Write(msgBuf)
	ck := crc32.ChecksumIEEE(dataBuf.Bytes())
	if readCk != ck {
		return nil, fmt.Errorf("crc校验错误 crc: %d, read crc: %d", ck, readCk)
	}

	var msg nt.BroadcastMsg
	err = json.Unmarshal(msgBuf, &msg)
	return &msg, err
}
