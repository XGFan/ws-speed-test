package main

import (
	"crypto/tls"
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
	"strconv"
	"strings"
	"sync"
	"time"
)

type LinkedList struct {
	head *Node
	size int
}

type Node struct {
	ip   string
	time int
	next *Node
}

func (list LinkedList) string() string {
	return list.head.next.string()
}

func (p Node) string() string {
	if p.next != nil {
		return strconv.Itoa(p.time) + "," + p.next.string()
	} else {
		return strconv.Itoa(p.time)
	}
}

func (list *LinkedList) Add(ip string, t int) {
	prev := list.head
	p := prev.next
	for p != nil && p.time < t {
		prev = p
		p = p.next
	}
	prev.next = &Node{ip, t, p}
	list.size++
}

func (list *LinkedList) Get(index int) *Node {
	p := list.head
	for i := 0; i < index+1 && p.next != nil; i++ {
		p = p.next
	}
	return p
}

func (list *LinkedList) Percent(per float32) int {
	index := int(float64(list.size) * float64(per) / float64(100))
	p := list.head
	for i := 0; i < index && p.next != nil; i++ {
		p = p.next
	}
	return p.time
}

func (list *LinkedList) Avg() int {
	var sum int64 = 0.0
	p := list.head.next
	for p != nil {
		sum = sum + int64(p.time)
		p = p.next
	}
	return int(sum / int64(list.size))
}

func main() {
	log.Println(runtime.GOMAXPROCS(0) * 2)
	//log.SetFlags(0)
	f, err := ioutil.ReadFile("cfip.txt")
	if err != nil {
		log.Fatal(err)
	}

	ips := make(chan string)
	go func() {
		split := strings.Split(string(f), "\n")
		for _, s := range split {
			ips <- s
		}
		close(ips)
	}()
	maxTcpingRoutines := 100
	waitTcping := sync.WaitGroup{}
	waitTcping.Add(maxTcpingRoutines)
	tcpingResult := make(chan Node)
	for i := 0; i < maxTcpingRoutines; i++ {
		go func() {
			for ip := range ips {
				i := tcping(ip, 443, time.Millisecond*300)
				tcpingResult <- Node{ip, i, nil}
			}
			waitTcping.Done()
		}()
	}
	go func() {
		waitTcping.Wait()
		close(tcpingResult)
	}()

	list := LinkedList{head: &Node{}}
	for res := range tcpingResult {
		list.Add(res.ip, res.time)
	}

	log.Println(list.Get(0))
	stIp := make(chan string)

	go func() {
		var max int
		if 100 < list.size {
			max = 100
		} else {
			max = list.size
		}
		for i := 0; i < max; i++ {
			node := list.Get(i)
			stIp <- node.ip
		}
		close(stIp)
	}()

	log.Println(list.Get(99))

	group := sync.WaitGroup{}
	group.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			for ip := range stIp {
				speedTest(ip, "jp.test4x.com")
			}
			group.Done()
		}()
	}
	group.Wait()
}

func speedTest(ip string, host string) {
	u := url.URL{Scheme: "wss", Host: ip, Path: "/test", RawQuery: "size=2"}
	//log.Printf("connecting to %s", u.String())

	start := time.Now().UnixNano()
	var header http.Header = map[string][]string{}
	header.Add("host", host)
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		//HandshakeTimeout: time.Second * 1,
	}
	c, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		//log.Println("dial:", err)
		return
	}
	defer c.Close()

	_, message, err := c.ReadMessage()
	if err != nil {
		//log.Println("read:", err)
		return
	}
	end := time.Now().UnixNano()
	kb := len(message) / 1024
	log.Printf("%s : %.2fkb/s", ip, float64(kb)/(float64(end-start)/1000000)*1000)
}

func tcping(host string, port int, timeout time.Duration) int {
	startTime := time.Now()
	target := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", target, timeout)
	endTime := time.Now()
	if err != nil {
		return math.MaxInt16
	} else {
		defer conn.Close()
		var t = float64(endTime.Sub(startTime)) / float64(time.Millisecond)
		//log.Printf(" addr=%s time=%4.2fms", conn.RemoteAddr().String(), t)
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
