package main

import (
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"
)

var (
	gWSSessionMgr *sessionMgr
	gHandler      *msgHandler
	gToken        uint64
	gUser         string
	gPassword     string
)

func main() {
	// https://pkg.go.dev/net/http/pprof
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	gLog = NewLogger(filepath.Dir(os.Args[0]), "openp2p", LvINFO, 1*1024*1024, LogFileAndConsole)
	rand.Seed(time.Now().UnixNano())
	if err := parseParams(); err != nil {
		gLog.Println(LvERROR, err)
		return
	}
	gHandler = &msgHandler{
		handlers: make(map[uint16]handlerInterface),
	}
	login := loginHandler{}
	gHandler.registerHandler(MsgLogin, &login)
	gHandler.registerHandler(MsgHeartbeat, &login)
	gHandler.registerHandler(MsgPush, &pushHandler{})
	gHandler.registerHandler(MsgRelay, &relayHandler{})
	gHandler.registerHandler(MsgReport, &reportHandler{})
	gHandler.registerHandler(MsgQuery, &queryHandler{})
	nat := natHandler{}
	gHandler.registerHandler(MsgNATDetect, &nat)
	for i := 0; i < 16; i++ {
		go gHandler.handleMessage()
	}
	initStun()
	gWSSessionMgr = NewSessionMgr()
	go gWSSessionMgr.run()
	runWeb()
	forever := make(chan bool)
	<-forever
}

func initStun() {
	go tcpServer(IfconfigPort1)
	go tcpServer(IfconfigPort2)
	_, err := newUDPServer(&net.UDPAddr{IP: net.IPv4zero, Port: UDPPort1})
	if err != nil {
		gLog.Println(LvERROR, "listen udp 1 failed:", err)
		return
	}
	_, err = newUDPServer(&net.UDPAddr{IP: net.IPv4zero, Port: UDPPort2})
	if err != nil {
		gLog.Println(LvERROR, "listen udp 2 failed:", err)
		return
	}
	gLog.Printf(LvINFO, "listen STUN UDP on: %d and %d", UDPPort1, UDPPort2)
}

func parseParams() error {
	gUser = os.Getenv("OPENP2P_USER")
	gPassword = os.Getenv("OPENP2P_PASSWORD")
	if gUser == "" || gPassword == "" {
		return ErrUserOrPwdNotSet
	} else {
		gToken = nodeNameToID(gUser + gPassword)
		gLog.Println(LvINFO, "TOKEN:", gToken)
	}
	JWTSecret = gUser + gPassword + "@openp2p.cn"
	return nil
}
