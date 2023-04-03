package state

import (
	"encoding/binary"
	"io"
)

func (t *Command) Marshal(w io.Writer) {
	Vl := uint64(len(t.V))
	var b [8]byte
	bs := b[:8]
	b[0] = byte(t.Op)
	bs = b[:1]
	w.Write(bs)
	bs = b[:8]
	binary.BigEndian.PutUint64(bs, uint64(t.K))
	w.Write(bs)
	binary.BigEndian.PutUint64(bs, uint64(Vl))
	w.Write(bs)
	w.Write([]byte(t.V))
}

func (t *Command) Unmarshal(r io.Reader) error {
	var b [8]byte
	bs := b[:8]

	bs = b[:1]
	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	t.Op = Operation(b[0])

	bs = b[:8]
	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	t.K = Key(binary.BigEndian.Uint64(bs))

	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	Vl := binary.BigEndian.Uint64(bs)

	data := make([]byte, Vl)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	t.V = Value(data)
	if Vl > 0 {
		// fmt.Println(t.Op, t.K, Vl, string(data))
	}
	return nil
}

func (t *Key) Marshal(w io.Writer) {
	var b [8]byte
	bs := b[:8]
	binary.BigEndian.PutUint64(bs, uint64(*t))
	w.Write(bs)
}

func (t *Value) Marshal(w io.Writer) {
	Vl := uint64(len(*t))
	var b [8]byte
	bs := b[:8]
	binary.BigEndian.PutUint64(bs, uint64(Vl))
	w.Write(bs)
	// fmt.Println("marshal, value length: ", bs)
	w.Write([]byte(*t))

}

func (t *Key) Unmarshal(r io.Reader) error {
	var b [8]byte
	bs := b[:8]
	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	*t = Key(binary.BigEndian.Uint64(bs))
	return nil
}

func (t *Value) Unmarshal(r io.Reader) error {
	var b [8]byte
	bs := b[:8]
	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	Vl := binary.BigEndian.Uint64(bs)
	data := make([]byte, Vl)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	V := Value(data)
	t = &V
	return nil
}
