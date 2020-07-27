## ws-speed-test

使用websocket进行测速，适用于cloudflare，可以用来选择合适的cf节点。



#### 服务端
```shell script
./stServer -addr 0.0.0.0:80
```

会开放3个url

```
/
主页，可以在网页测试websocket速度
/test
websocket接口，接受一个size参数，单位为MB，服务端会在握手之后向客户端发送该大小的随机数据
/204
仅仅返回204，用于测试http的RT时间
```



#### 客户端

```shell script
./stClient -host yourDomainName -size 5
Usage of stClient:
  -d int
    	max goroutine to download (default 4)
  -dn int
    	result count from download (default 20)
  -file string
    	ip list file (default "cfip.txt")
  -host string
    	remote service address (default "jp.test4x.com")
  -p int
    	max goroutine to ping (default 50)
  -pn int
    	result count from ping (default 50)
  -size int
    	test packet size(MB) (default 5)
```

会先读取cf的ip列表，通过50个协程通过204接口，查找RT时间最快的50个IP，然后再从这50个IP中寻找Download速度最快的30个IP信息。

```
2020/07/27 23:15:51 addr: 104.22.216.15	speed: 804.25KB/s	http-ping: 622ms
2020/07/27 23:15:51 addr: 104.21.26.15	speed: 800.19KB/s	http-ping: 650ms
2020/07/27 23:15:51 addr: 104.24.164.15	speed: 798.86KB/s	http-ping: 640ms
2020/07/27 23:15:51 addr: 104.20.140.15	speed: 797.47KB/s	http-ping: 645ms
2020/07/27 23:15:51 addr: 104.26.211.15	speed: 796.28KB/s	http-ping: 638ms、
……
```

