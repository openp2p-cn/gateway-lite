package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type wssSession struct {
	conn            *websocket.Conn
	writeCh         chan []byte
	rspCh           chan []byte
	running         bool
	node            string
	shareBandWidth  int
	user            string
	token           uint64
	nodeID          uint64
	natType         int
	os              string
	lanIP           string
	hasUPNPorNATPMP int
	hasIPv4         int
	IPv4            string
	IPv6            string
	mac             string
	version         string
	majorVer        int
	activeTime      time.Time
	relayTime       time.Time
	failNodes       sync.Map
}

// var allSessions sync.Map

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type sessionMgr struct {
	allSessions    map[uint64]*wssSession
	allSessionsMtx sync.RWMutex
	pushPermission sync.Map
}

func NewSessionMgr() *sessionMgr {
	return &sessionMgr{allSessions: make(map[uint64]*wssSession)}
}

func (mgr *sessionMgr) run() {
	r := gin.New()
	r.Use(gin.Recovery())
	r.StaticFS("/files", gin.Dir("/var/openp2p", false))
	r.GET("/openp2p/v1/login", mgr.handleLogin)

	go r.RunTLS(fmt.Sprintf(":%d", WsPort), "api.crt", "api.key")
	statTimer := time.NewTicker(time.Minute)

	for {
		select {
		case <-statTimer.C:
			if time.Now().Minute() == 0 && time.Now().Hour()%6 == 0 {
				mgr.allSessionsMtx.RLock()
				gLog.Println(LvINFO, "总览", fmt.Sprintf("在线客户端数量:%d\n连接汇总:%v", len(mgr.allSessions), connectResult))
				mgr.allSessionsMtx.RUnlock()
			}
		}
	}
}

func (mgr *sessionMgr) handleLogin(c *gin.Context) {
	node := c.Query("node")
	tokenStr := c.Query("token")
	version := c.Query("version")
	natTypeStr := c.Query("nattype")
	bwStr := c.Query("sharebandwidth")
	shareBw, _ := strconv.Atoi(bwStr)
	natType, _ := strconv.Atoi(natTypeStr)
	token, _ := strconv.ParseUint(tokenStr, 10, 64)
	gLog.Printf(LvINFO, "handle login:node=%s,natType=%s,bw=%d,version=%s", node, natTypeStr, shareBw, version)
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		gLog.Printf(LvERROR, "%s:accept web socket failed:", node)
		return
	}

	ipv4 := strings.Split(ws.RemoteAddr().String(), ":")[0]
	sess := &wssSession{
		node:           node,
		token:          token,
		shareBandWidth: shareBw,
		conn:           ws,
		IPv4:           ipv4,
		writeCh:        make(chan []byte, 10),
		rspCh:          make(chan []byte, 10),
		running:        true,
		nodeID:         nodeNameToID(node),
		natType:        natType,
		version:        version,
		activeTime:     time.Now(),
		relayTime:      time.Now().AddDate(0, 0, -1),
	}
	if len(sess.node) < MinNodeNameLen {
		gLog.Println(LvERROR, ErrNodeTooShort)
		sess.writeSync(MsgLogin, 0, &LoginRsp{
			Error:  500,
			Detail: ErrNodeTooShort.Error(),
			Ts:     0,
		})
		return
	}
	if token != gToken {
		gLog.Println(LvERROR, "Invalid token:", token)
		sess.writeSync(MsgLogin, 0, &LoginRsp{
			Error:  600,
			Detail: "Invalid token",
			Ts:     0,
		})
	}
	mgr.allSessionsMtx.Lock()
	mgr.allSessions[sess.nodeID] = sess
	mgr.allSessionsMtx.Unlock()
	gLog.Println(LvDEBUG, sess)
	gLog.Println(LvINFO, "client login success:", sess.node)
	// TODO: use epoll for large numbers of connections
	go sess.writeLoop()
	go sess.readLoop()
	sess.write(MsgLogin, 0, &LoginRsp{
		Error: 0,
		Ts:    time.Now().Unix(),
		User:  sess.user,
		Token: sess.token,
		Node:  sess.node,
	})
}

func (sess *wssSession) writeLoop() {
	for sess.running {
		msg, ok := <-sess.writeCh
		if !ok {
			continue
		}
		if err := sess.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			gLog.Printf(LvERROR, "session %s write failed:%s", sess.node, err)
			break
		}
	}
}

func (sess *wssSession) readLoop() {
	for {
		sess.conn.SetReadDeadline(time.Now().Add(NetworkHeartbeatTime + time.Second*5))
		t, msg, err := sess.conn.ReadMessage()
		if err != nil {
			gLog.Printf(LvERROR, "%s,%d read error %s, offline", sess.node, sess.nodeID, err)
			sess.running = false
			close(sess.writeCh)
			gWSSessionMgr.allSessionsMtx.Lock()
			// sometimes the new ws session has replaced the old one, DO NOT delete it.
			if s, ok := gWSSessionMgr.allSessions[sess.nodeID]; ok && s == sess {
				delete(gWSSessionMgr.allSessions, sess.nodeID)
			}
			gWSSessionMgr.allSessionsMtx.Unlock()
			gWSSessionMgr.pushPermission.Delete(sess.nodeID)
			break
		}
		if t == websocket.BinaryMessage {
			MsgQueue <- &msgContext{
				sess: sess,
				msg:  msg,
			}
		}
	}
}

func (sess *wssSession) close() error {
	return nil
}

func (sess *wssSession) write(mainType uint16, subType uint16, packet interface{}) error {
	msg, err := newMessage(mainType, subType, packet)
	if err == nil && sess.running {
		sess.writeCh <- msg
	}
	return nil
}

func (sess *wssSession) writeSync(mainType uint16, subType uint16, packet interface{}) error {
	msg, err := newMessage(mainType, subType, packet)
	if err == nil && sess.running {
		if err := sess.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			gLog.Printf(LvERROR, "session %s write failed:%s", sess.node, err)
		}
	}
	return nil
}

func (sess *wssSession) writeBuff(buff []byte) error {
	if sess.running {
		sess.writeCh <- buff
	}
	return nil
}
