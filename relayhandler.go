package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/openp2p-cn/totp"
)

type relayHandler struct {
}

func (h *relayHandler) handleMessage(ctx *msgContext) error {
	head := openP2PHeader{}
	err := binary.Read(bytes.NewReader(ctx.msg[:openP2PHeaderSize]), binary.LittleEndian, &head)
	if err != nil {
		return err
	}
	reqSess, ok := ctx.sess.(*wssSession)
	if !ok {
		gLog.Println(LvERROR, "interface conversion error")
		return errors.New("interface conversion error")
	}
	switch head.SubType {
	case MsgRelayNodeReq:
		// gLogger.Println(LvINFO, string(msg))
		// TODO: find one cone node NOT randomly. Group node, bandwidth, latency, online stable...
		req := RelayNodeReq{}
		err = json.Unmarshal(ctx.msg[openP2PHeaderSize:], &req)
		if err != nil {
			gLog.Printf(LvERROR, "wrong RelayNodeReq:%s", err)
			return err
		}
		var relayNodeName string
		var relayNodeToken uint64
		relayMode := "private"
		peerNodeID := nodeNameToID(req.PeerNode)
		gWSSessionMgr.allSessionsMtx.RLock()
		defer gWSSessionMgr.allSessionsMtx.RUnlock()
		peerSess, ok := gWSSessionMgr.allSessions[peerNodeID]
		if !ok {
			gLog.Printf(LvERROR, "request relay node error: %s offline", req.PeerNode)
			return nil
		}
		peerIP := peerSess.IPv4
		var relaySess *wssSession
		gLog.Printf(LvINFO, "searching relay node: %s:%s---%s:%s", reqSess.node, reqSess.IPv4, req.PeerNode, peerIP)
		// count fail try
		failNum := 0
		reqSess.failNodes.Range(func(k, v interface{}) bool {
			ts := v.(time.Time)
			if ts.After(time.Now().Add(-time.Hour)) { // count within 1h failure
				failNum++
			}
			return true
		})
		if failNum > MaxDirectTry {
			gLog.Println(LvINFO, "force search PUBLIC IP NODE")
		}
		// find usableNodes
		// find relay node in this user
		for _, sess := range gWSSessionMgr.allSessions {
			if reqSess.majorVer != sess.majorVer {
				continue
			}
			// exclude requester itself
			if sess.user != reqSess.user || sess.natType == NATSymmetric {
				continue
			}
			// these two network could not connect directly, so filter them
			if sess.IPv4 == reqSess.IPv4 || sess.IPv4 == peerIP {
				continue
			}
			// // filter failNodes
			// if _, ok := reqSess.failNodes.Load(sess.nodeID); ok {
			// 	continue
			// }
			if relaySess == nil {
				relaySess = sess
			} else if sess.relayTime.Before(relaySess.relayTime) { // find the most idle node
				relaySess = sess
			}
			// relaytime > 30mins idle long enough, break the range
			if sess.relayTime.Before(time.Now().Add(-time.Minute * 30)) {
				break
			}
		}

		if relaySess != nil {
			relayNodeName = relaySess.node
			t := totp.TOTP{Step: totp.RelayTOTPStep}
			relayNodeToken = t.Gen(relaySess.token, time.Now().Unix())
			relaySess.relayTime = time.Now()
			if relaySess.user != reqSess.user {
				relayMode = "public"
			}
			gLog.Printf(LvINFO, "got %s relay node %s:%d", relayMode, relayNodeName, relayNodeToken)
		} else {
			gLog.Printf(LvINFO, "no available relay node")
		}
		rsp := RelayNodeRsp{
			Mode:       relayMode,
			RelayName:  relayNodeName,
			RelayToken: relayNodeToken}
		reqSess.write(MsgRelay, MsgRelayNodeRsp, &rsp)

	default:
		return nil
	}
	return nil
}
