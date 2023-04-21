package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"time"
)

type UDPServer struct {
	conn *net.UDPConn
	la   *net.UDPAddr
	tm   *time.Timer
}
type UDPSession struct {
	conn *net.UDPConn
	ra   *net.UDPAddr
}

func newUDPServer(la *net.UDPAddr) (*UDPServer, error) {
	conn, err := net.ListenUDP("udp", la)
	if err != nil {
		return nil, err
	}
	if la == nil {
		la, _ = net.ResolveUDPAddr("udp", conn.LocalAddr().String())
	}
	sess := &UDPServer{
		conn: conn,
		la:   la,
	}
	sess.start()
	return sess, nil
}

func (sess *UDPServer) start() {
	sess.tm = time.NewTimer(time.Second)
	sess.tm.Stop()
	go sess.run()
}

func (sess *UDPServer) run() {
	for {
		msg := make([]byte, 1600)
		len, ra, e := sess.conn.ReadFromUDP(msg)
		MsgQueue <- &msgContext{
			sess: &UDPSession{
				conn: sess.conn,
				ra:   ra,
			},
			msg: msg[:len],
		}
		if e != nil {
			break
		}
	}
}

func (sess *UDPServer) stop() {
	sess.conn.Close()
}

func (sess *UDPSession) write(mainType uint16, subType uint16, packet interface{}) error {
	data, err := json.Marshal(packet)
	if err != nil {
		gLog.Printf(LvERROR, "marshal data failed:%s", err)
		return err
	}
	// gLog.Println(LvINFO, "write packet:", string(data))
	head := openP2PHeader{
		uint32(len(data)),
		mainType,
		subType,
	}
	headBuf := new(bytes.Buffer)
	err = binary.Write(headBuf, binary.LittleEndian, head)
	if err != nil {
		return err
	}
	writeBytes := append(headBuf.Bytes(), data...)
	_, err = sess.conn.WriteToUDP(writeBytes, sess.ra)
	return err

}

func (sess *UDPSession) close() error {
	return nil
}
