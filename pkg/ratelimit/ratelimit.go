package ratelimit

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"github.com/Alwandy/chapter1/pkg/ringbuffer"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Handler struct {
	event map[string]Event
}

type Event struct {
	events []events
}

type events struct {
	timestamps 	time.Time
	path		string
}

var wg sync.WaitGroup

func Init(){
	h := Handler{}
	h.process()
}
//process will start reading the sample log
func (h *Handler) process() {
	file, err := os.Open("test/sample.log")
	if err != nil {
		log.Fatalf("[ERROR] %s", err)
	}
	defer file.Close() // Important to close file when done reading file
	lines := bufio.NewScanner(file)
	m := make(map[string]Event)
	for lines.Scan() {
		s := strings.Split(lines.Text(), " ")
		s[3] = strings.Replace(s[3], "[", "", 1)
		s[4] = strings.Replace(s[4], "]", "", 1)
		timestamp, _ := time.Parse("02/Jan/2006:15:04:05", fmt.Sprintf("%s", s[3]))
		t := events{timestamp, s[6] }
		if _, ok := m[s[0]]; ok {
			res := append(m[s[0]].events, t)
			m[s[0]] = Event{events: res}
		} else {
			m[s[0]] = Event{events: []events{t}}
		}

	}

	h.event = m
	h.rateLimit()

	if err := lines.Err(); err != nil {
		log.Fatalf("[ERROR] %s", err)
	}
}

func (h *Handler) rateLimit(){
	for ip, _ := range h.event {
		go h.rateLimitExceeded(ip)
	}
	wg.Wait()
}

//rateLimitExceeded will use the IP as the key on the map we created in previous method and filter through all timestamps
//There's a BUG that after the unban occurs it will continue where X stopped and ban again, this might not be a bug
//As the test sample might be incorrect as the IP shouldn't not have been able to connect but it still does even after the ratelimiting or their response should been 429
//But in sample they're still 200 / 301
func (h *Handler) rateLimitExceeded(ip string) {
	wg.Add(1) // Added waitgroup so we can run the ratelimiting concurrently
	defer wg.Done()
	banlist := make(map[string]bool)
	//Rules
	rule := ringBufPool.Get().(*ringbuffer.RingBufferRateLimiter)
	rule2 := ringBufPool.Get().(*ringbuffer.RingBufferRateLimiter)
	rule3 := ringBufPool.Get().(*ringbuffer.RingBufferRateLimiter)
	rule.Initialize(100, 10*time.Minute)
	rule2.Initialize(40, 1*time.Minute)
	rule3.Initialize(20, 10*time.Minute)
	for x, _ := range h.event[ip].events {

		//Rules Reserve
		rule.Reserve(h.event[ip].events[x].timestamps)
		rule2.Reserve(h.event[ip].events[x].timestamps)
		if h.event[ip].events[x].path == "/login" {
		rule3.Reserve(h.event[ip].events[x].timestamps)
		}
		if rule3.Count(h.event[ip].events[x].timestamps) >= rule3.MaxEvents() && !banlist[ip] {
			banlist[ip] = true
			writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "BAN")
			fmt.Printf("BANNED IP %s on RULE 3\n", ip)
			go unban(ip, 120, banlist)
		} else if rule2.Count(h.event[ip].events[x].timestamps) >= rule2.MaxEvents() && !banlist[ip] {
			banlist[ip] = true
			writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "BAN")
			fmt.Printf("BANNED IP %s on RULE 2\n", ip)
			go unban(ip, 10, banlist)
		} else if rule.Count(h.event[ip].events[x].timestamps) >= rule.MaxEvents() && !banlist[ip] {
			banlist[ip] = true
			writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "BAN")
			fmt.Printf("BANNED IP %s on RULE 1\n", ip)
			go unban(ip, 60, banlist)
		}
	}
}

func writeToCsv(ip, timestamp, action string) {
	// read the file
	f, err := os.OpenFile("ban.csv", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	column := []string{timestamp, action, ip}
	w := csv.NewWriter(f)
	w.Write(column)
	w.Flush()
}

func unban(ip string, dur time.Duration, banlist map[string]bool) map[string]bool{
	time.Sleep(dur * time.Minute)
	writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "UNBAN")
	banlist[ip] = false
	return banlist
}

// ringBufPool reduces allocations from unneeded rate limiters.
var ringBufPool = sync.Pool{
	New: func() interface{} {
		return new(ringbuffer.RingBufferRateLimiter)
	},
}
