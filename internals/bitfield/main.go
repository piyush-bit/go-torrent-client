package bitfield

import "math/bits"

type Bitfield []byte

func NewBitfield(size int32) Bitfield {
	return make(Bitfield, (size-1)/8+1)
}

func (b Bitfield) SetAll() {
	for i := range b {
		b[i] = 0xFF
	}
}

func (b Bitfield) IsAllSet(bitfieldLength int32) bool {
    totalBytes := (bitfieldLength + 7) / 8

    for i := int32(0); i < totalBytes; i++ {
        isLast := (i == totalBytes-1)

        if !isLast {
            if b[i] != 0xFF {
                return false
            }
            continue
        }

        remainingBits := bitfieldLength % 8
        if remainingBits == 0 {
            return b[i] == 0xFF
        }

        for j := 0; j < int(remainingBits); j++ {
            if b[i]&(1<<(7-j)) == 0 {
                return false
            }
        }
    }
    return true
}


func (b Bitfield) Set(index int32) {
    b[index/8] |= 1 << (7 - (index % 8))
}


func (b Bitfield) Clear(index int32) {
    b[index/8] &^= 1 << (7 - (index % 8))
}

func (b Bitfield) Test(index int32) bool {
    return b[index/8]&(1<<(7-(index%8))) != 0
}



func (b Bitfield) FirstSetBit(offset int32) int32 {
	n := len(b)
	for i := int(offset / 8); i < n; i++ {
		if b[i] != 0 {
			return int32(i*8 + bits.TrailingZeros8(b[i]))
		}
	}
	return -1
}
