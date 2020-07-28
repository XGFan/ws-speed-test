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

func main() {
	var host = *flag.String("host", "jp.test4x.com", "remote service address")
	var file = *flag.String("file", "cfip.txt", "ip list file")
	var size = *flag.Int("size", 5, "test packet size(MB)")
	var pingRoutine = *flag.Int("p", 50, "max goroutine to ping")
	var pingCount = *flag.Int("pn", 50, "result count from ping")
	var downloadRoutine = *flag.Int("d", 4, "max goroutine to download")
	var downloadCount = *flag.Int("dn", 20, "result count from download")
	flag.Parse()
	runtime.GOMAXPROCS(runtime.GOMAXPROCS(0) * 2)
	ipChan := getIp(file, host)
	ipAndTime := channelSort(channelMap(ipChan, pingRoutine, ping(host)), pingCount)
	ipAndTimeAndSpeed := channelMap(ipAndTime, downloadRoutine, speed(host, size))
	list := channelToList(ipAndTimeAndSpeed, downloadCount)
	for _, node := range list {
		log.Println(node)
	}
}

func getIp(fileName, host string) chan *Node {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Printf("can not read %s, use dns instead\n", fileName)
		return findIp(host)
	} else {
		return readIp(bytes)
	}
}

func readIp(ipLines []byte) chan *Node {
	ips := make(chan *Node)
	split := strings.Split(string(ipLines), "\n")
	go func() {
		for _, s := range split {
			ips <- &Node{
				ip: s,
			}
		}
		close(ips)
	}()
	return ips
}

func findIp(host string) chan *Node {
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

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeA)
	msg.RecursionDesired = true

	m := make(map[string]bool)
	for _, n := range ns {
		r, _, err := c.Exchange(msg, net.JoinHostPort(n, config.Port))
		if r == nil {
			log.Printf("error: %+v\n", err)
			continue
		}
		if r.Rcode != dns.RcodeSuccess {
			log.Printf("error: %+v\n", r)
			continue
		}
		for _, a := range r.Answer {
			if dns.Type(a.Header().Rrtype).String() == "A" {
				m[a.(*dns.A).A.String()] = true
			}
		}
	}
	ips := make([]*Node, len(m))
	for k := range m {
		ips = append(ips, &Node{
			ip: k,
		})
	}
	return listToChannel(ips)
}

func websocketTest(ws *websocket.Dialer, ip string, size int) float64 {
	u := url.URL{Scheme: "wss", Host: ip, Path: "/test", RawQuery: fmt.Sprintf("size=%d", size)}
	start := time.Now().UnixNano()
	var header http.Header = map[string][]string{}
	header.Set("host", ws.TLSClientConfig.ServerName)
	c, _, err := ws.Dial(u.String(), header)
	if err != nil {
		log.Printf("addr=%s FAIL", ip)
		return 0
	}
	defer c.Close()
	_, message, err := c.ReadMessage()
	if err != nil {
		log.Printf("addr=%s FAIL", ip)
		return 0
	}
	end := time.Now().UnixNano()
	kb := len(message) / 1024
	speed := float64(kb) / (float64(end-start) / 1000000) * 1000
	log.Printf("addr=%s speed=%.2fkb/s", ip, speed)
	return speed
}

func httpTest(client *http.Client, ip string) int {
	request, _ := http.NewRequest("GET", fmt.Sprintf("https://%s/204", ip), nil)
	request.Header.Set("host", client.Transport.(*http.Transport).TLSClientConfig.ServerName)
	startTime := time.Now()
	_, err := client.Do(request)
	if err != nil {
		log.Printf(" addr=%s FAIL", ip)
		return math.MaxInt16
	} else {
		var t = float64(time.Now().Sub(startTime)) / float64(time.Millisecond)
		log.Printf(" addr=%s time=%4.2fms", ip, t)
		return int(t)
	}
}

func channelMap(c chan *Node, routine int, f func(x *Node)) chan *Node {
	result := make(chan *Node)
	wg := sync.WaitGroup{}
	wg.Add(routine)
	for i := 0; i < routine; i++ {
		go func() {
			for node := range c {
				f(node)
				result <- node
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(result)
	}()
	return result
}

func channelSort(c chan *Node, size int) chan *Node {
	list := channelToList(c, size)
	return listToChannel(list)
}

func listToChannel(list List) chan *Node {
	result := make(chan *Node)
	go func() {
		for _, item := range list {
			result <- item
		}
		close(result)
	}()
	return result
}

func channelToList(c chan *Node, size int) List {
	list := List{}
	for item := range c {
		list = append(list, item)
	}
	sort.Sort(list)
	max := list.Len()
	if max > size {
		max = size
	}
	list = list[:max]
	return list
}

func ping(host string) func(n *Node) {
	httpClient := &http.Client{
		Timeout: 1000 * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         host,
			},
		},
	}
	return func(n *Node) {
		n.time = httpTest(httpClient, n.ip)
	}
}

func speed(host string, size int) func(n *Node) {
	ws := &websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true, ServerName: host},
		HandshakeTimeout: time.Second * 2,
	}
	return func(n *Node) {
		n.speed = websocketTest(ws, n.ip, size)
	}
}
