package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type UserInfo struct {
	Password string
	Token    string
}

type TokenInfo struct {
	User    string
	Expired time.Time
}

type deviceInfo struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	IP         string `json:"ip,omitempty"`
	IPv6       string `json:"ipv6,omitempty"`
	NatType    string `json:"natType,omitempty"`
	Bandwidth  string `json:"bandwidth,omitempty"`
	LanIP      string `json:"lanip,omitempty"`
	MAC        string `json:"mac,omitempty"`
	OS         string `json:"os,omitempty"`
	IsActive   int    `json:"isActive,omitempty"`
	Version    string `json:"version,omitempty"`
	Remark     string `json:"remark,omitempty"`
	Removed    int    `json:"removed,omitempty"`
	Activetime string `json:"activetime,omitempty"`
	Addtime    string `json:"addtime,omitempty"`
	IsUpdate   bool   `json:"isUpdate,omitempty"`
}

type deviceList struct {
	Nodes     []deviceInfo `json:"nodes" binding:"required"`
	LatestVer string       `json:"latestVer,omitempty"`
}

var JWTSecret string

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// just token not jwt
		auth, ok := c.Request.Header["Authorization"]
		if !ok {
			c.String(http.StatusUnauthorized, "")
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(auth[0], &OpenP2PClaim{}, func(token *jwt.Token) (interface{}, error) {
			// Don't forget to validate the alg is what you expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(JWTSecret), nil
		})
		if err != nil {
			gLog.Println(LvERROR, "Parse token error:", err)
			c.String(http.StatusUnauthorized, "")
			c.Abort()
			return
		}
		claims, ok := token.Claims.(*OpenP2PClaim)
		if ok && token.Valid {
			fmt.Println(claims)
			if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
				c.String(http.StatusUnauthorized, "")
				c.Abort()
				return
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Next()
	}
}

func runWeb() {
	router := gin.Default()
	router.GET("/api/v1/devices", listDevices, AuthMiddleware())
	router.GET("/api/v1/device/:name/restart", restartDevice, AuthMiddleware())
	user := router.Group("/api/v1/user")
	user.POST("/login", webLogin)
	device := router.Group("/api/v1/device")
	device.Use(AuthMiddleware())
	device.GET("/:name/apps", listApps)
	device.POST("/:name/app", editApp)
	device.POST("/:name/switchapp", switchApp)
	router.RunTLS(":10008", "api.crt", "api.key")
	// router.Run(":10008")
}

func webLogin(c *gin.Context) {
	data, _ := c.GetRawData()
	req := ProfileInfo{}
	err := json.Unmarshal(data, &req)
	if err != nil {
		log.Println("wrong loginReq")
		return
	}
	gLog.Println(LvINFO, "wechatLogin:", req.User)
	if req.User != gUser || req.Password != gPassword {
		c.String(http.StatusBadRequest, "登录失败")
		log.Println("authorize error:")
		return
	}
	// new token
	claim := OpenP2PClaim{
		User:         req.User,
		InstallToken: fmt.Sprintf("%d", gToken),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().AddDate(0, 0, 1).Unix(),
			// ExpiresAt: time.Now().Add(time.Second * 60).Unix(),  // test
			Issuer:   "openp2p.cn",
			IssuedAt: time.Now().Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(JWTSecret))
	if err != nil {
		fmt.Println(tokenString, JWTSecret, err)
		return
	}

	log.Println("authorize ok:")
	c.JSON(http.StatusOK, gin.H{
		"token":     tokenString,
		"nodeToken": fmt.Sprintf("%d", gToken),
		"error":     0,
	})
}

func listDevices(c *gin.Context) {
	gWSSessionMgr.allSessionsMtx.RLock()
	defer gWSSessionMgr.allSessionsMtx.RUnlock()
	// TODO: no query latestVer each request.
	var latestVer string
	nodes := deviceList{}
	nodes.LatestVer = latestVer
	// list online devices
	for _, sess := range gWSSessionMgr.allSessions {
		data := deviceInfo{}
		data.Name = sess.node
		data.NatType = fmt.Sprintf("%d", sess.natType)
		data.Bandwidth = fmt.Sprintf("%d", sess.shareBandWidth)
		data.IP = sess.IPv4
		data.IPv6 = sess.IPv6
		data.LanIP = sess.lanIP
		data.MAC = sess.mac
		data.OS = sess.os
		data.Version = sess.version
		data.Activetime = sess.activeTime.Local().String()
		data.IsActive = 1
		data.IsUpdate = true
		data.ID = fmt.Sprintf("%d", nodeNameToID(data.Name))
		nodes.Nodes = append(nodes.Nodes, data)
	}
	// TODO: list offline devices in mysql
	log.Println("get devices:", nodes)
	c.JSON(http.StatusOK, nodes)
}

func listApps(c *gin.Context) {

	nodeName := c.Param("name")
	uuid := nodeNameToID(nodeName)
	gLog.Println(LvINFO, nodeName, " update")
	gWSSessionMgr.allSessionsMtx.Lock()
	sess, ok := gWSSessionMgr.allSessions[uuid]
	gWSSessionMgr.allSessionsMtx.Unlock()

	if !ok {
		gLog.Printf(LvERROR, "listTunnel %d error: peer offline", uuid)
		c.JSON(http.StatusOK, gin.H{"error": 1, "detail": "device offline"})
		return
	}
	sess.write(MsgPush, MsgPushReportApps, nil)
	// TODO verify token
	// wait for the channel at most 5 seconds
	select {
	case msg := <-sess.rspCh:
		c.String(http.StatusOK, "%s", msg)
	case <-time.After(ClientAPITimeout):
		// Timed out after 5 seconds!
		log.Printf("listTunnel %d timeout.", uuid)
		c.JSON(http.StatusNotFound, gin.H{"error": 9, "detail": "timeout"})
	}
}

func editApp(c *gin.Context) {
	nodeName := c.Param("name")
	uuid := nodeNameToID(nodeName)
	gWSSessionMgr.allSessionsMtx.Lock()
	sess, ok := gWSSessionMgr.allSessions[uuid]
	gWSSessionMgr.allSessionsMtx.Unlock()

	if !ok {
		gLog.Printf(LvERROR, "editApp %d error: peer offline", uuid)
		c.JSON(http.StatusOK, gin.H{"error": 1, "detail": "device offline"})
		return
	}
	app := AppInfo{}
	buf, _ := c.GetRawData()
	err := json.Unmarshal(buf, &app)
	if err != nil {
		gLog.Printf(LvERROR, "wrong AppInfo:%s", err)
		c.String(http.StatusNotAcceptable, "")
		return
	}
	gLog.Println(LvINFO, "edit app:", app)
	sess.write(MsgPush, MsgPushEditApp, app)
	c.String(http.StatusOK, "")
}

func switchApp(c *gin.Context) {
	nodeName := c.Param("name")
	uuid := nodeNameToID(nodeName)
	gWSSessionMgr.allSessionsMtx.Lock()
	sess, ok := gWSSessionMgr.allSessions[uuid]
	gWSSessionMgr.allSessionsMtx.Unlock()

	if !ok {
		gLog.Printf(LvERROR, "switchApp %d error: peer offline", uuid)
		c.JSON(http.StatusOK, gin.H{"error": 1, "detail": "device offline"})
		return
	}
	app := AppInfo{}
	buf, _ := c.GetRawData()
	err := json.Unmarshal(buf, &app)
	if err != nil {
		gLog.Printf(LvERROR, "wrong AppInfo:%s", err)
		c.String(http.StatusNotAcceptable, "")
		return
	}
	gLog.Println(LvINFO, "switchApp app:", app)
	sess.write(MsgPush, MsgPushSwitchApp, app)
	c.String(http.StatusOK, "")
}

func init() {
}

type OpenP2PClaim struct {
	User         string `json:"user,omitempty"`
	InstallToken string `json:"installToken,omitempty"`
	jwt.StandardClaims
}

func restartDevice(c *gin.Context) {
	nodeName := c.Param("name")
	uuid := nodeNameToID(nodeName)
	gLog.Println(LvINFO, nodeName, " restart")
	gWSSessionMgr.allSessionsMtx.Lock()
	sess, ok := gWSSessionMgr.allSessions[uuid]
	gWSSessionMgr.allSessionsMtx.Unlock()
	if !ok {
		gLog.Printf(LvERROR, "push to %s error: peer offline", nodeName)
		c.JSON(http.StatusOK, gin.H{"error": 1, "detail": "device offline"})
		return
	}
	sess.write(MsgPush, MsgPushRestart, nil)
	c.JSON(http.StatusOK, gin.H{"error": 0})
}
