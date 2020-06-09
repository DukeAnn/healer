package healer

import (
	"bytes"
	"io/ioutil"

	"github.com/pierrec/lz4"
	//"github.com/bkaradzic/go-lz4"
)

type LZ4Compressor struct {
}

func (c *LZ4Compressor) Compress(value []byte) ([]byte, error) {
	reader := lz4ReaderPool.Get().(*lz4.Reader)
	defer lz4ReaderPool.Put(reader)

	reader.Reset(bytes.NewReader(value))
	return ioutil.ReadAll(reader)
}
