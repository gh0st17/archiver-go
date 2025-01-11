package compressor_test

import (
	"archiver/compressor"
	"bytes"
	"crypto/md5"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestNop(t *testing.T) {
	runTest(t, compressor.Nop, compressor.Level(0))
}

func TestGzip(t *testing.T) {
	for cl := compressor.Level(-2); cl <= 9; cl++ {
		runTest(t, compressor.GZip, cl)
	}
}

func TestLzw(t *testing.T) {
	runTest(t, compressor.LempelZivWelch, compressor.Level(-1))
}

func TestZlib(t *testing.T) {
	for cl := compressor.Level(-2); cl <= 9; cl++ {
		runTest(t, compressor.ZLib, cl)
	}
}

func runTest(t *testing.T, ct compressor.Type, cl compressor.Level) {
	const dataSize = 20 * 1024 * 1024

	var (
		decompBuf     = bytes.NewBuffer(nil)
		compBuf       = bytes.NewBuffer(nil)
		rng           = rand.New(rand.NewSource(time.Now().Unix()))
		lowEntropyVal [32]byte
		inMD5, outMD5 MD5hash
		err           error
		c             *compressor.Writer
		d             *compressor.Reader
	)

	switch ct {
	case compressor.LempelZivWelch, compressor.Nop:
		t.Log("Testing", ct, "compressor")
	default:
		t.Log("Testing", ct, "compressor with", cl, "level")
	}

	for i := range lowEntropyVal {
		lowEntropyVal[i] = byte(rng.Intn(256))
	}

	for i := 0; i < dataSize; i++ {
		k := rng.Intn(len(lowEntropyVal))
		decompBuf.Write([]byte{lowEntropyVal[k]})
	}
	inMD5 = hashBytes(decompBuf.Bytes())

	if c, err = compressor.NewWriter(ct, compBuf, cl); err != nil {
		t.Fatal(err)
	}

	if _, err = decompBuf.WriteTo(c); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}

	if d, err = compressor.NewReader(ct, compBuf); err != nil {
		t.Fatal(err)
	}

	if _, err = decompBuf.ReadFrom(d); err != nil {
		t.Fatal(err)
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	outMD5 = hashBytes(decompBuf.Bytes())

	if !compareMD5(inMD5, outMD5) {
		t.Errorf("Expected %s got %s", inMD5, outMD5)
		t.Fail()
	}
}

func hashBytes(b []byte) MD5hash { return MD5hash(md5.New().Sum(b)) }

func compareMD5(buf1, buf2 MD5hash) bool {
	for i := range buf1 {
		if buf1[i] != buf2[i] {
			return false
		}
	}

	return true
}

type MD5hash []byte

func (s MD5hash) String() string {
	var buf bytes.Buffer

	for _, b := range s {
		buf.WriteString(fmt.Sprintf("%02x", b))
	}

	return buf.String()
}
