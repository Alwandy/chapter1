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
// Normally I wouldn't execute the code in a PKG, I would create a new class but for simplicity, I've done this
type Handler struct {
	event map[string]Event
	banlist map[string]bool
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
	fi, err := file.Stat()
	if err != nil {
		log.Fatalf("[ERROR] %s", err)
	}
	if fi.Size() <= 0 {
		log.Printf("[INFO] File %s is empty", file.Name())
	}
	defer file.Close() // Important to close file when done reading file
	lines := bufio.NewScanner(file)
	m := make(map[string]Event)

	for lines.Scan() {
		s := strings.Split(lines.Text(), " ")
		s[3] = strings.Replace(s[3], "[", "", 1)
		ip := s[0]
		date := s[3]
		path := s[6]

		timestamp, _ := time.Parse("02/Jan/2006:15:04:05", fmt.Sprintf("%s", date))
		t := events{timestamp, path }
		if _, ok := m[ip]; ok {
			res := append(m[ip].events, t)
			m[ip] = Event{events: res}
		} else {
			m[ip] = Event{events: []events{t}}
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
	h.banlist = make(map[string]bool)
	//Rules
	rule := ringBufPool.Get().(*ringbuffer.RingBufferRateLimiter)
	rule2 := ringBufPool.Get().(*ringbuffer.RingBufferRateLimiter)
	rule3 := ringBufPool.Get().(*ringbuffer.RingBufferRateLimiter)
	rule.Initialize(100, 10*time.Minute)
	rule2.Initialize(40, 1*time.Minute)
	rule3.Initialize(20, 10*time.Minute)
	for x, _ := range h.event[ip].events {
		if h.banlist[ip] {
			t := time.Tick(1 * time.Minute)
			for {
				select {
				case <- t:
					continue
				}
			}
		}
		//Rules Reserve
		rule.Reserve(h.event[ip].events[x].timestamps)
		rule2.Reserve(h.event[ip].events[x].timestamps)
		if h.event[ip].events[x].path == "/login" {
			rule3.Reserve(h.event[ip].events[x].timestamps)
		}

		if rule3.Count(h.event[ip].events[x].timestamps) >= rule3.MaxEvents() && !h.banlist[ip] {
			h.banlist[ip] = true
			writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "BAN")
			log.Printf("[INFO] BANNED IP %s on RULE 3\n", ip)
			h.unban(ip, 120)
		} else if rule2.Count(h.event[ip].events[x].timestamps) >= rule2.MaxEvents() && !h.banlist[ip] {
			h.banlist[ip] = true
			writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "BAN")
			log.Printf("[INFO] BANNED IP %s on RULE 2\n", ip)
			h.unban(ip, 10)
		} else if rule.Count(h.event[ip].events[x].timestamps) >= rule.MaxEvents() && !h.banlist[ip] {
			h.banlist[ip] = true
			writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "BAN")
			log.Printf("[INFO] BANNED IP %s on RULE 1\n", ip)
			h.unban(ip, 60)
		}
	}
	wg.Wait()
}

func writeToCsv(ip, timestamp, action string) {
	// read the file
	f, err := os.OpenFile("ban.csv", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("[ERROR] %s", err)
		return
	}
	column := []string{timestamp, action, ip}
	w := csv.NewWriter(f)
	w.Write(column)
	w.Flush()
}

func (h *Handler) unban(ip string, dur time.Duration) {
	t := time.Tick(dur * time.Minute)
	for {
		select {
			case <- t:
				writeToCsv(ip, strconv.FormatInt(time.Now().Unix(), 10), "UNBAN")
				h.banlist[ip] = false
		}
	}
}

// ringBufPool reduces allocations from unneeded rate limiters.
var ringBufPool = sync.Pool{
	New: func() interface{} {
		return new(ringbuffer.RingBufferRateLimiter)
	},
}
