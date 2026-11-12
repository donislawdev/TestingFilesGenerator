package wav

import "encoding/binary"

// The label rides in a LIST INFO chunk between the format and the data.

// infoChunk builds a LIST INFO chunk carrying the label in the software
// field, which is where a player looks for it.
func infoChunk(label string) []byte {
	text := append([]byte(label), 0)
	if len(text)%2 == 1 {
		text = append(text, 0)
	}
	inner := make([]byte, 0, 4+chunkHeader+len(text))
	inner = append(inner, []byte("INFO")...)
	inner = append(inner, []byte("ISFT")...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(text)))
	inner = append(inner, n[:]...)
	inner = append(inner, text...)

	out := make([]byte, 0, chunkHeader+len(inner))
	out = append(out, []byte("LIST")...)
	binary.LittleEndian.PutUint32(n[:], uint32(len(inner)))
	out = append(out, n[:]...)
	out = append(out, inner...)
	return out
}
