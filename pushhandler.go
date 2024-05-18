package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/openp2p-cn/totp"
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
		rsp, _ := json.Marshal(PushRsp{Error: 1, Detail: "push denied: unSupported msg "})
		rsp = append(ctx.msg[:openP2PHeaderSize+PushHeaderSize], rsp...)
		_ = copy(rsp, encodeHeader(head.MainType, MsgPushRsp, uint32(len(rsp)-openP2PHeaderSize)))
		fromSess.writeBuff(rsp)
		return errors.New("push denied")
	}
	gWSSessionMgr.allSessionsMtx.Lock()
	toSess, ok := gWSSessionMgr.allSessions[pushHead.To]
	gWSSessionMgr.allSessionsMtx.Unlock()
	if !ok {
		gLog.Printf(LvERROR, "%d push to %d error: peer offline", pushHead.From, pushHead.To)
		rsp, _ := json.Marshal(PushRsp{Error: 1, Detail: "peer offline"})
		rsp = append(ctx.msg[:openP2PHeaderSize+PushHeaderSize], rsp...)
		_ = copy(rsp, encodeHeader(head.MainType, MsgPushRsp, uint32(len(rsp)-openP2PHeaderSize)))
		fromSess.writeBuff(rsp)
		return errors.New("peer offline")
	}
	if head.SubType == MsgPushConnectReq {
		// verify user/password
		req := PushConnectReq{}
		err := json.Unmarshal(ctx.msg[openP2PHeaderSize+PushHeaderSize:], &req)
		if err != nil {
			gLog.Printf(LvERROR, "wrong MsgPushConnectReq:%s", err)
			return err
		}
		gLog.Printf(LvINFO, "%s is connecting to %s...", req.From, toSess.node)
		t := totp.TOTP{Step: totp.RelayTOTPStep}
		if !(t.Verify(req.Token, toSess.token, time.Now().Unix()) || (toSess.token == req.FromToken)) { // (toSess.token == req.FromToken) is deprecated
			gLog.Printf(LvERROR, "%s --- %s MsgPushConnectReq push denied", req.From, toSess.node)
			rsp, _ := json.Marshal(PushRsp{Error: 1, Detail: "MsgPushConnectReq push denied"})
			rsp = append(ctx.msg[:openP2PHeaderSize+PushHeaderSize], rsp...)
			_ = copy(rsp, encodeHeader(head.MainType, MsgPushRsp, uint32(len(rsp)-openP2PHeaderSize)))
			fromSess.writeBuff(rsp)
			return errors.New("push denied")
		}
		// cache push permission 60s
		gWSSessionMgr.pushPermission.Store(pushHead.From, &pushNodeInfo{pushHead.To, time.Now().Add(time.Minute)})
		gWSSessionMgr.pushPermission.Store(pushHead.To, &pushNodeInfo{pushHead.From, time.Now().Add(time.Minute)})
		// TODO: clear cache
	} else if isByPassMsg(head.SubType) {
		gLog.Println(LvDEBUG, "sync app key")
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
			rsp, _ := json.Marshal(PushRsp{Error: 1, Detail: "push denied"})
			rsp = append(ctx.msg[:openP2PHeaderSize+PushHeaderSize], rsp...)
			_ = copy(rsp, encodeHeader(head.MainType, MsgPushRsp, uint32(len(rsp)-openP2PHeaderSize)))
			fromSess.writeBuff(rsp)
			return errors.New("push denied")
		}
	}
	toSess.writeBuff(ctx.msg)
	rsp, _ := json.Marshal(PushRsp{Error: 0, Detail: fmt.Sprintf("push msgType %d,%d to %d ok", head.MainType, head.SubType, pushHead.From)})
	rsp = append(ctx.msg[:openP2PHeaderSize+PushHeaderSize], rsp...)
	_ = copy(rsp, encodeHeader(head.MainType, MsgPushRsp, uint32(len(rsp)-openP2PHeaderSize)))
	fromSess.writeBuff(rsp)
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
	return msgType == MsgPushAPPKey
}
