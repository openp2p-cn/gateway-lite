package main

import (
	"bytes"
	"encoding/binary"
	"sync"
)

// MsgQueue is global message queue
var MsgQueue chan (*msgContext)

type msgContext struct {
	sess session
	msg  []byte
}

type handlerInterface interface {
	handleMessage(ctx *msgContext) error
}

type msgHandler struct {
	handlersMtx sync.RWMutex
	handlers    map[uint16]handlerInterface
}

func (h *msgHandler) registerHandler(msgType uint16, hi handlerInterface) error {
	h.handlersMtx.Lock()
	defer h.handlersMtx.Unlock()
	//TODO: duplicated msgType
	h.handlers[msgType] = hi
	return nil
}

func (h *msgHandler) handleMessage() error {
	for {
		select {
		case ctx := <-MsgQueue:
			// TODO: parse header multi times
			head := openP2PHeader{}
			err := binary.Read(bytes.NewReader(ctx.msg[:openP2PHeaderSize]), binary.LittleEndian, &head)
			if err != nil {
				continue
			}
			h, ok := h.handlers[head.MainType]
			if ok {
				err = h.handleMessage(ctx)
				if err != nil {
					gLog.Println(LvERROR, err)
				}
			}
		}
	}
}

func init() {
	MsgQueue = make(chan *msgContext, 1000)
}
