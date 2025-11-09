package message

import (
	"encoding/binary"
)

type MessageID uint8

const (
	MsgChoke         MessageID = 0
	MsgUnchoke       MessageID = 1
	MsgInterested    MessageID = 2
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4
	MsgBitfield      MessageID = 5
	MsgRequest       MessageID = 6
	MsgPiece         MessageID = 7
	MsgCancel        MessageID = 8
	MsgKeepAlive     MessageID = 9
)

type Message struct {
	ID   MessageID
	Data []byte
}

func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}
	length := len(m.Data) + 1
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf, uint32(length))
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Data)
	return buf
}

func (m *Message) GetPieceIndex() int32 {
	return int32(binary.BigEndian.Uint32(m.Data))
}

func Choke() *Message {
	return &Message{
		ID:   MsgChoke,
		Data: nil,
	}
}

func Unchoke() *Message {
	return &Message{
		ID:   MsgUnchoke,
		Data: nil,
	}
}

func Interested() *Message {
	return &Message{
		ID:   MsgInterested,
		Data: nil,
	}
}

func NotInterested() *Message {
	return &Message{
		ID:   MsgNotInterested,
		Data: nil,
	}
}

func Have(index int32) *Message {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(index))
	return &Message{
		ID:   MsgHave,
		Data: buf,
	}
}

func Bitfield(bitfield []byte) *Message {
	return &Message{
		ID:   MsgBitfield,
		Data: bitfield,
	}
}

func Request(index int32, begin int32, length int32) *Message {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf, uint32(index))
	binary.BigEndian.PutUint32(buf[4:8], uint32(begin))
	binary.BigEndian.PutUint32(buf[8:12], uint32(length))
	return &Message{
		ID:   MsgRequest,
		Data: buf,
	}
}

func Piece(index int32, begin int32, data []byte) *Message {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf, uint32(index))
	binary.BigEndian.PutUint32(buf[4:], uint32(begin))
	binary.BigEndian.PutUint32(buf[8:], uint32(len(data)))
	return &Message{
		ID:   MsgPiece,
		Data: append(buf, data...),
	}
}

func Cancel(index int32, begin int32, length int32) *Message {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf, uint32(index))
	binary.BigEndian.PutUint32(buf[4:], uint32(begin))
	binary.BigEndian.PutUint32(buf[8:], uint32(length))
	return &Message{
		ID:   MsgCancel,
		Data: buf,
	}
}
