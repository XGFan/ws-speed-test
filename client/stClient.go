package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/miekg/dns"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Node struct {
	ip    string
	time  int
	speed float64
}

func (p Node) String() string {
	return fmt.Sprintf("addr: %s\tspeed: %.2fKB/s\thttp-ping: %dms", p.ip, p.speed, p.time)
}

type List []*Node

func (list List) Len() int { return len(list) }
func (list List) Less(i, j int) bool {
	if list[i].speed < list[j].speed {
		return false
	} else if list[i].speed > list[j].speed {
		return true
	} else {
		return list[i].time < list[j].time
	}
}
func (list List) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func readIps(name, host string) []string {
	f, err := ioutil.ReadFile(name)
	if err != nil {
		log.Printf("can not read %s, use dns instead\n", name)
		return findIp(host)
	} else {
		ips := make([]string, 0)

		split := strings.Split(string(f), "\n")
		for _, s := range split {
			ips = append(ips, s)
		}
		return ips
	}
}

func main() {
	var host = flag.String("host", "jp.test4x.com", "remote service address")
	var file = flag.String("file", "cfip.txt", "ip list file")
	var size = flag.Int("size", 5, "test packet size(MB)")
	var pingRoutine = flag.Int("p", 50, "max goroutine to ping")
	var pingCount = flag.Int("pn", 50, "result count from ping")
	var downloadRoutine = flag.Int("d", 4, "max goroutine to download")
	var downloadCount = flag.Int("dn", 20, "result count from download")
	flag.Parse()
	runtime.GOMAXPROCS(runtime.GOMAXPROCS(0) * 2)
	ips := readIps(*file, *host)
	list := ping(ips, *host, *pingRoutine)
	speed(list[:*pingCount], *host, *size, *downloadRoutine)
	sort.Sort(list)
	for i := 0; i < *downloadCount; i++ {
		node := list[i]
		log.Println(node)
	}
}

func speed(ips List, host string, size int, routine int) {
	ws := &websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true, ServerName: host},
		HandshakeTimeout: time.Second * 2,
	}
	group := sync.WaitGroup{}
	group.Add(routine)
	var ops int32 = -1
	for i := 0; i < routine; i++ {
		go func() {
			for {
				index := int(atomic.AddInt32(&ops, 1))
				if index < ips.Len() {
					ips[index].speed = speedTest(ws, ips[index].ip, size)
				} else {
					break
				}
			}
			group.Done()
		}()
	}
	group.Wait()
}

func (list *List) toChannel(maxCount int) chan *Node {
	stIp := make(chan *Node)
	go func() {
		var max int
		if maxCount < list.Len() {
			max = maxCount
		} else {
			max = list.Len()
		}
		sort.Sort(list)
		for i := 0; i < max; i++ {
			node := (*list)[i]
			stIp <- node
		}
		close(stIp)
	}()
	return stIp
}
func ping(ips []string, host string, routine int) List {
	httpClient := &http.Client{
		Timeout: 1500 * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         host,
			},
		},
	}

	waitHttpPing := sync.WaitGroup{}
	waitHttpPing.Add(routine)
	httpPingResult := make(chan Node)
	var ops int32 = -1
	for i := 0; i < routine; i++ {
		go func() {
			for {
				index := int(atomic.AddInt32(&ops, 1))
				if index < len(ips) {
					ip := ips[index]
					i := httpGet(httpClient, ip)
					httpPingResult <- Node{ip, i, 0}
				} else {
					break
				}
			}
			waitHttpPing.Done()
		}()
	}
	go func() {
		waitHttpPing.Wait()
		close(httpPingResult)
	}()
	list := List{}
	for res := range httpPingResult {
		list = append(list, &Node{ip: res.ip, time: res.time})
	}
	sort.Sort(list)
	return list
}

func speedTest(ws *websocket.Dialer, ip string, size int) float64 {
	u := url.URL{Scheme: "wss", Host: ip, Path: "/test", RawQuery: fmt.Sprintf("size=%d", size)}
	start := time.Now().UnixNano()
	var header http.Header = map[string][]string{}
	header.Set("host", ws.TLSClientConfig.ServerName)
	c, _, err := ws.Dial(u.String(), header)
	if err != nil {
		return 0
	}
	defer c.Close()
	_, message, err := c.ReadMessage()
	if err != nil {
		return 0
	}
	end := time.Now().UnixNano()
	kb := len(message) / 1024
	speed := float64(kb) / (float64(end-start) / 1000000) * 1000
	log.Printf("%s : %.2fkb/s", ip, speed)
	return speed
}

func httpGet(client *http.Client, ip string) int {
	request, _ := http.NewRequest("GET", fmt.Sprintf("https://%s/204", ip), nil)
	request.Header.Set("host", client.Transport.(*http.Transport).TLSClientConfig.ServerName)
	startTime := time.Now()
	_, err := client.Do(request)
	if err != nil {
		return math.MaxInt16
	} else {
		var t = float64(time.Now().Sub(startTime)) / float64(time.Millisecond)
		log.Printf(" addr=%s time=%4.2fms", ip, t)
		return int(t)
	}
}

func findIp(name string) []string {
	ns := []string{
		"223.5.5.5",       //ali
		"223.6.6.6",       //ali
		"180.76.76.76",    //baidu
		"114.114.114.114", //114
		"119.29.29.29",    //dnspod
		"182.254.116.116", //dnspod
		"117.50.10.10",    //onedns
		"52.80.52.52",     //onedns
		"8.8.4.4",         //google
		"8.8.8.8",         //google
		"1.1.1.1",         //cf
		"1.0.0.1",         //cf
		"101.226.4.6",     //dnspai
		"218.30.118.6",    //dnspai
		"185.222.222.222", //dns.sb
		"185.184.222.222", //dns.sb
		"208.67.222.222",  //OpenDNS
		"185.222.222.222", //OpenDNS
		"1.2.4.8",         //CNNIC
		"210.2.4.8",       //CNNIC
	}
	config := dns.ClientConfig{Servers: ns, Port: "53", Timeout: 10}
	c := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	m.RecursionDesired = true

	ips := make([]string, 0)
	for _, n := range ns {
		r, _, err := c.Exchange(m, net.JoinHostPort(n, config.Port))
		if r == nil {
			log.Printf("error: %+v\n", err)
			continue
		}

		if r.Rcode != dns.RcodeSuccess {
			log.Printf("error: %+v\n", r)
			continue
		}
		// Stuff must be in the answer section
		for _, a := range r.Answer {
			if dns.Type(a.Header().Rrtype).String() == "A" {
				x := a.(*dns.A)
				flag := true
				for _, s := range ips {
					if s == x.A.String() {
						flag = false
						break
					}
				}
				if flag {
					ips = append(ips, x.A.String())
				}
			}
		}
	}
	return ips
}
