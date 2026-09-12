package content

import (
	"encoding/binary"
	"testing"
)

func TestBitmap565OrientationAndChannels(t *testing.T) {
	b := Bitmap565([]uint16{0xf800, 0x07e0, 0x001f, 0xffff}, 2)
	if string(b[:2]) != "BM" || binary.LittleEndian.Uint32(b[2:]) != uint32(len(b)) {
		t.Fatal("bad BMP header")
	}
	want := []byte{255, 0, 0, 255, 255, 255, 0, 0, 0, 0, 255, 0, 255, 0, 0, 0}
	for i, v := range want {
		if b[54+i] != v {
			t.Fatalf("pixel byte %d = %d, want %d", i, b[54+i], v)
		}
	}
}
