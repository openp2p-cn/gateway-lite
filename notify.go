package main

const chanSize int = 100000
const threadNum int = 16

var (
	onlineNodes chan string
)

func PushNotifyChan(node string) {
	if len(onlineNodes) < chanSize-100 {
		onlineNodes <- node
	}
}

func init() {
	onlineNodes = make(chan string, chanSize)
	for i := 0; i < threadNum; i++ {
		go notifyLoop()
	}
}

func notifyLoop() {
	for node := range onlineNodes {
		// notify all src nodes in apps where dstnode=node
		dealOne(node)
	}
}

func dealOne(node string) {
	gWSSessionMgr.allSessionsMtx.Lock()
	for _, sess := range gWSSessionMgr.allSessions {
		sess.write(MsgPush, MsgPushDstNodeOnline, &PushDstNodeOnline{Node: node})
	}
	gWSSessionMgr.allSessionsMtx.Unlock()
}
