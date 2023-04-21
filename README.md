# Get started
Run your self-hosting openp2p gateway with **`4 STEP`**.
It demonstrates how to YOUR-PC1--->YOUR-PC2

## 1. Run gateway program
### Run with docker(Recommend)
```
docker run -d --restart always --net=host -e OPENP2P_USER=YOUR-NAME -e OPENP2P_PASSWORD=YOUR-PASSWORD  --mount type=bind,src=/etc/localtime,dst=/etc/localtime,ro --name openp2p-gateway openp2pcn/openp2p-gateway:latest
```

### Run directly
```
export GOPROXY=https://goproxy.io,direct
go mod tidy
go build
export OPENP2P_USER=YOUR-NAME 
export OPENP2P_PASSWORD=YOUR-PASSWORD
./openp2p-gateway

```

## 2. Login 
with user+password return jwt+token
```
curl --insecure "https://YOUR-SERVER:10008/api/v1/user/login" -X POST -d '
{
    "user": "YOUR-NAME",
    "password": "YOUR-PASSWORD"
}'

response:

{
    "error":0,
    "nodeToken":"xxxxxxxxxxxxxxxx",  // for client install
    "token":"xxxxxxxxxxxxxxxx"  // for api call
}
```

## 3. Install client
download openp2p client on https://github.com/openp2p-cn/openp2p/releases

on PC1
```
wget https://github.com/openp2p-cn/openp2p/releases/download/v3.6.11/openp2p3.6.11.linux-amd64.tar.gz
tar xvf openp2p3.6.11.linux-amd64.tar.gz 
./openp2p -node YOUR-PC1 -serverhost YOUR-SERVER -token YOUR-TOKEN
```
-serverhost: is your server domain or ip

-token: is the nodeToken in STEP 2 login response

on PC2
```
wget https://github.com/openp2p-cn/openp2p/releases/download/v3.6.11/openp2p3.6.11.linux-amd64.tar.gz
tar xvf openp2p3.6.11.linux-amd64.tar.gz 
./openp2p -node YOUR-PC2 -serverhost YOUR-SERVER -token YOUR-TOKEN
```

on YOUR-SERVER, when 2 node can't p2p connect, they need a relay node, so install a openp2p client as relay node on your server is recommand.
```
wget https://github.com/openp2p-cn/openp2p/releases/download/v3.6.11/openp2p3.6.11.linux-amd64.tar.gz
tar xvf openp2p3.6.11.linux-amd64.tar.gz 
./openp2p -node YOUR-SERVER -serverhost YOUR-SERVER -token YOUR-TOKEN
```

## 4. New app
Call api with jwt token in http header

Return 2XX is success, otherwise failed

Example:  
YOUR-PC1:localhost:23389--->YOUR-PC2:localhost:22
```
curl --insecure "https://YOUR-SERVER:10008/api/v1/device/YOUR-PC1/app" -X POST -H 'Authorization: YOUR-TOKEN' -d '
{
        "appName": "RemoteDesktop",
        "protocol": "tcp",
        "srcPort": 23389,
        "peerNode": "YOUR-PC2",
        "dstHost": "localhost",
        "dstPort": 22
}'
```

YOUR-TOKEN is the token in STEP 2 login response

## API reference

### List all apps
```
curl --insecure "https://YOUR-SERVER:10008/api/v1/device/YOUR-PC1/apps" -H 'Authorization: YOUR-TOKEN' 
```

### Edit app
//protocol0+srcPort0 is the old p2papp's id
edit the tcp+23389 app

local:23389--->YOUR-PC2:localhost:22  change to
local:55555--->YOUR-PC2:localhost:22

```
curl --insecure "https://YOUR-SERVER:10008/api/v1/device/YOUR-PC1/app" -X POST -H 'Authorization: YOUR-TOKEN' -d '
{
        "appName": "RemoteSSH",
        "protocol": "tcp",
        "srcPort": 55555,
        "protocol0": "tcp",  
        "srcPort0": 23389,
        "peerNode": "YOUR-PC2",
        "dstHost": "localhost",
        "dstPort": 22
}'
```

### Delele app
```
curl --insecure "https://YOUR-SERVER:10008/api/v1/device/YOUR-PC1/app" -X POST -H 'Authorization: YOUR-TOKEN' -d '
{
        "protocol0": "tcp",  
        "srcPort0": 55555,
        "dstPort": 22
}'
```

### Enable/Disable app

Enable
```
curl --insecure "https://YOUR-SERVER:10008/api/v1/device/YOUR-PC1/switchapp" -X POST -H 'Authorization: YOUR-TOKEN' -d '
{
        "protocol": "tcp",
        "srcPort": 55555,
        "enabled": 1
}'
```

Disable
```
curl --insecure "https://YOUR-SERVER:10008/api/v1/device/YOUR-PC1/switchapp" -X POST -H 'Authorization: YOUR-TOKEN' -d '
{
        "protocol": "tcp",
        "srcPort": 55555,
        "enabled": 0
}'
```

## self-signed cert
```
openssl req -newkey rsa \
            -x509 \
            -sha256 \
            -days 3650 \
            -nodes \
            -out api.crt \
            -keyout api.key \
            -subj "/C=CN/ST=BJ/L=BJ/O=Security/OU=IT Department/CN=openp2p.cn"

```