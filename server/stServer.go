package main

import (
	"flag"
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var addr = flag.String("addr", "0.0.0.0:10022", "http service address")

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return true
}}

func test(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Sec-Websocket-Key") != "" && !strings.Contains(r.Header.Get("Connection"), "upgrade") {
		r.Header.Add("Connection", "upgrade")
	}
	start := time.Now().UnixNano()
	log.Printf("--------")
	for s, values := range r.Header {
		log.Printf("%s: %+v\n", s, values)
	}
	size, e := strconv.Atoi(r.URL.Query().Get("size"))
	if e != nil {
		size = 1
		e = nil
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("fail on upgrade", err)
		return
	}
	defer c.Close()
	bytes := make([]byte, 1024*1024*size)
	rand.Read(bytes)
	e = c.WriteMessage(websocket.BinaryMessage, bytes)
	if e != nil {
		log.Printf("fail on write %dMB\n", size)
		log.Println(e)
		return
	}
	e = c.WriteControl(websocket.CloseMessage, make([]byte, 0), time.Time{})

	if e != nil {
		log.Println("fail on close", e)
		return
	}
	end := time.Now().UnixNano()

	log.Printf("write %dMB in %dms\n", size, (end-start)/1000000)
}

func empty(w http.ResponseWriter,r *http.Request){
	w.WriteHeader(204)
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>ws speed test</title>
    <meta charset="utf-8">
    <script>
        window.addEventListener("load", function (evt) {
            const output = document.getElementById("output");
            const input = document.getElementById("input");
            let ws;
            const print = function (message) {
                let d = document.createElement("div");
                d.textContent = message;
                output.appendChild(d);
            };
            document.getElementById("test").onclick = function (evt) {
                if (ws) {
                    return false;
                }
                let url
                if (location.protocol == "http:") {
                    url = "ws://" + location.host + "/test?size=10"
                } else {
                    url = "wss://" + location.host + "/test?size=10"
                }
                ws = new WebSocket(url);
                let start
                let end
                ws.onopen = function (evt) {
                    print("Start");
                    start = Date.now()
                }
                ws.onclose = function (evt) {
                    end = Date.now()
                    const speed = input.value * 1024 * 1000 / (end - start)
                    print("Speed: " + speed.toFixed(2) + "KB/S");
                    ws = null;
                }
                ws.onerror = function (evt) {
                    print("ERROR: " + evt.data);
                }
                return false;
            };
        });
    </script>
</head>
<body>
<table>
    <tr>
        <td valign="top" width="45%">
            <p>Click "Test" to create a connection to the server And test the speed</p>
            <form>
                <input id="input" type="number" value="10">
                <button id="test">Test</button>
            </form>
        </td>
        <td valign="top" width="10%"></td>
        <td valign="top" width="45%">
            <div id="output"></div>
        </td>
    </tr>
</table>
</body>
</html>`))
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/test", test)
	http.HandleFunc("/204", empty)

	http.HandleFunc("/", home)
	log.Fatal(http.ListenAndServe(*addr, nil))
}