package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"
)

type loginHandler struct {
}

func (h *loginHandler) handleMessage(ctx *msgContext) error {
	head := openP2PHeader{}
	err := binary.Read(bytes.NewReader(ctx.msg[:openP2PHeaderSize]), binary.LittleEndian, &head)
	if err != nil {
		return err
	}
	wsSess, ok := ctx.sess.(*wssSession)
	if !ok {
		gLog.Println(LvERROR, "interface conversion error")
		return errors.New("interface conversion error")
	}
	switch head.MainType {
	case MsgLogin:
		// gLogger.Println(LvINFO, string(msg))
		rsp := LoginRsp{}
		err = json.Unmarshal(ctx.msg[openP2PHeaderSize:], &rsp)
		if err != nil {
			wsSess.close()
			gLog.Printf(LvERROR, "wrong login response:%s", err)
			return err
		}
		if rsp.Error != 0 {
			wsSess.close()
			gLog.Printf(LvERROR, "login error:%d", rsp.Error)
		} else {
			gLog.Printf(LvINFO, "%s login ok", wsSess.node)
		}
	case MsgHeartbeat:
		wsSess.activeTime = time.Now()
		wsSess.writeBuff(ctx.msg)
		// gLog.Printf(LvINFO, "%s heartbeat ok", wsSess.node)
	default:
		return nil
	}
	return nil
}
