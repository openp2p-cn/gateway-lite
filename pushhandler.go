package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type pushHandler struct {
}

type pushNodeInfo struct {
	to        uint64
	expiredAt time.Time
}

func (h *pushHandler) handleMessage(ctx *msgContext) error {
	head := openP2PHeader{}
	err := binary.Read(bytes.NewReader(ctx.msg[:openP2PHeaderSize]), binary.LittleEndian, &head)
	if err != nil {
		return err
	}
	pushHead := PushHeader{}
	err = binary.Read(bytes.NewReader(ctx.msg[openP2PHeaderSize:openP2PHeaderSize+PushHeaderSize]), binary.LittleEndian, &pushHead)
	if err != nil {
		return err
	}
	fromSess, ok := ctx.sess.(*wssSession)
	if !ok {
		gLog.Println(LvERROR, "interface conversion error")
		return errors.New("interface conversion error")
	}
	gLog.Printf(LvDEBUG, "%d push to %d", pushHead.From, pushHead.To)
	if !isSupportMsg(head.SubType) {
		rsp := PushRsp{Error: 1, Detail: "push denied: unSupported msg "}
		fromSess.write(head.MainType, MsgPushRsp, rsp)
		return errors.New("push denied")
	}
	gWSSessionMgr.allSessionsMtx.Lock()
	toSess, ok := gWSSessionMgr.allSessions[pushHead.To]
	gWSSessionMgr.allSessionsMtx.Unlock()
	if !ok {
		gLog.Printf(LvERROR, "%d push to %d error: peer offline", pushHead.From, pushHead.To)
		rsp := PushRsp{Error: 1, Detail: "peer offline"}
		fromSess.write(head.MainType, MsgPushRsp, rsp)
		return errors.New("peer offline")
	}
	if isByPassMsg(head.SubType) {
		switch head.SubType {
		case MsgPushConnectReq:
			gLog.Printf(LvINFO, "%s is connecting to %s...", fromSess.node, toSess.node)
			/*
				// verify user/password
				t := totp.TOTP{Step: totp.RelayTOTPStep}
				if !(t.Verify(req.Token, toSess.token, time.Now().Unix()) || (toSess.token == req.FromToken)) { // (toSess.token == req.FromToken) is deprecated
					gLog.Printf(LvERROR, "%s --- %s MsgPushConnectReq push denied", req.From, toSess.node)
					rsp := PushRsp{Error: 1, Detail: "MsgPushConnectReq push denied"}
					fromSess.write(head.MainType, MsgPushRsp, rsp)
					return errors.New("push denied")
				}
			*/
		case MsgPushConnectRsp:
			// check rsp.Error for permission
			var rsp PushConnectRsp
			err := json.Unmarshal(ctx.msg[openP2PHeaderSize+PushHeaderSize:], &rsp)
			if err != nil {
				gLog.Printf(LvERROR, "wrong MsgPushConnectRsp:%s", err)
				return err
			}
			if rsp.Error&0xFF == 0 { // allow more success code
				// cache push permission 60s
				gWSSessionMgr.pushPermission.Store(pushHead.To, &pushNodeInfo{pushHead.From, time.Now().Add(time.Minute)})
				gWSSessionMgr.pushPermission.Store(pushHead.From, &pushNodeInfo{pushHead.To, time.Now().Add(time.Minute)})
				// TODO: clear cache
			} else {
				gLog.Printf(LvWARN, "%s --- %s connect error %d: %s", rsp.To, rsp.From, rsp.Error, rsp.Detail)
			}
		case MsgPushAPPKey:
			gLog.Println(LvDEBUG, "sync app key")
		default:
			gLog.Println(LvWARN, "unknown by pass msg ", head.SubType)
		}
	} else {
		// verify push permission
		// verify from as key
		i, ok := gWSSessionMgr.pushPermission.Load(pushHead.From)
		granted := false
		if ok {
			nodeInfo := i.(*pushNodeInfo)
			if nodeInfo.expiredAt.After(time.Now()) {
				granted = true
			} else {
				gWSSessionMgr.pushPermission.Delete(pushHead.From)
			}
		}
		if !granted {
			gLog.Printf(LvERROR, "%d --- %d push denied", pushHead.From, pushHead.To)
			rsp := PushRsp{Error: 1, Detail: "push denied"}
			fromSess.write(head.MainType, MsgPushRsp, rsp)
			return errors.New("push denied")
		}
	}
	toSess.writeBuff(ctx.msg)
	rsp := PushRsp{Error: 0, Detail: fmt.Sprintf("push msgType %d,%d to %d ok", head.MainType, head.SubType, pushHead.From)}
	fromSess.write(head.MainType, MsgPushRsp, rsp)
	return nil
}

func isSupportMsg(msgType uint16) bool {
	if msgType == MsgPushRsp ||
		msgType == MsgPushConnectReq ||
		msgType == MsgPushConnectRsp ||
		msgType == MsgPushHandshakeStart ||
		msgType == MsgPushAddRelayTunnelReq ||
		msgType == MsgPushAddRelayTunnelRsp ||
		msgType == MsgPushUnderlayConnect ||
		msgType == MsgPushAPPKey {
		return true
	}
	return false
}

func isByPassMsg(msgType uint16) bool {
	return msgType == MsgPushConnectReq ||
		msgType == MsgPushConnectRsp ||
		msgType == MsgPushAPPKey
}
