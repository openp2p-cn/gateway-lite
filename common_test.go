package main

import (
	"log"
	"testing"
)

func TestAESCBC(t *testing.T) {
	for packetSize := 1; packetSize <= 8192; packetSize++ {
		log.Println("test packetSize=", packetSize)
		data := make([]byte, packetSize)
		for i := 0; i < packetSize; i++ {
			data[i] = byte('0' + i%10)
		}
		p2pEncryptBuf := make([]byte, len(data)+PaddingSize)
		inBuf := make([]byte, len(data)+PaddingSize)
		copy(inBuf, data)
		cryptKey := []byte("0123456789ABCDEF")
		sendBuf, err := encryptBytes(cryptKey, p2pEncryptBuf, inBuf, len(data))
		if err != nil {
			t.Errorf("encrypt packet failed:%s", err)
		}
		log.Printf("encrypt data len=%d\n", len(sendBuf))

		decryptBuf := make([]byte, len(sendBuf))
		outBuf, err := decryptBytes(cryptKey, decryptBuf, sendBuf, len(sendBuf))
		if err != nil {
			t.Errorf("decrypt packet failed:%s", err)
		}
		// log.Printf("len=%d,content=%s\n", len(outBuf), outBuf)
		log.Printf("decrypt data len=%d\n", len(outBuf))
		log.Println("validate")
		for i := 0; i < len(outBuf); i++ {
			if outBuf[i] != byte('0'+i%10) {
				t.Error("validate failed")
			}
		}
		log.Println("validate ok")
	}

}

func TestNetInfo(t *testing.T) {
	log.Println(netInfo())
}

func wrapTestCompareVersion(t *testing.T, v1 string, v2 string, result int) {
	if compareVersion(v1, v2) == result {
		// t.Logf("compare version %s %s ok\n", v1, v2)
	} else {
		t.Errorf("compare version %s %s fail\n", v1, v2)
	}
}
func TestCompareVersion(t *testing.T) {
	// test =
	wrapTestCompareVersion(t, "0.73.0.1234", "0.73.0.1234", EQUAL)
	wrapTestCompareVersion(t, "0.73.0", "0.73.0", EQUAL)
	wrapTestCompareVersion(t, "0.73", "0.73", EQUAL)
	wrapTestCompareVersion(t, "1.0.0.1234", "1.0.0.1234", EQUAL)
	wrapTestCompareVersion(t, "1.5.0", "1.5.0", EQUAL)
	// test >
	wrapTestCompareVersion(t, "0.73.0.1235", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "0.73.1.1234", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "0.74.0.1234", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "0.173.0.1234", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "0.73.0.1234", "0.73.0", GREATER)
	wrapTestCompareVersion(t, "1.73.0.1234", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "1.73.0.1234", "0.73.0", GREATER)
	wrapTestCompareVersion(t, "1.73.0.1234", "0.73", GREATER)
	wrapTestCompareVersion(t, "10.73.0.1234", "9.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "1.0.0", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "1.0", "0.73.0.1234", GREATER)
	wrapTestCompareVersion(t, "1", "0.73.0.1234", GREATER)
	// test <
	wrapTestCompareVersion(t, "0.73.0.1234", "0.73.0.1235", LESS)
	wrapTestCompareVersion(t, "0.73.0.1234", "0.73.1.1234", LESS)
	wrapTestCompareVersion(t, "0.73.0.1234", "0.74.0.1234", LESS)
	wrapTestCompareVersion(t, "0.73.0.1234", "0.173.0.1234", LESS)
	wrapTestCompareVersion(t, "0.73.0.1234", "1.73.0.1234", LESS)
	wrapTestCompareVersion(t, "9.73.0.1234", "10.73.0.1234", LESS)
	wrapTestCompareVersion(t, "1.4.2", "1.5.0", LESS)
	wrapTestCompareVersion(t, "", "1.5.0", LESS)

}
