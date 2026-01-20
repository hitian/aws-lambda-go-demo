package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"sync"
	"strings"
	"time"

	"github.com/apex/gateway/v2"
	"github.com/gin-gonic/gin"
	"github.com/oschwald/geoip2-golang"
)

const (
	storeSizeLimit  = 30 * 1024 * 1024 //10 Mb
	storeExpireTime = 5                //minute
)

var (
	version   = "debug"
	startTime = time.Now()
)

func routerEngine() *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Define your handlers
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World!")
	})
	r.GET("/version", func(c *gin.Context) {
		c.String(http.StatusOK, "version: "+version+", "+runtime.Version())
	})
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/ip", func(c *gin.Context) {
		clientAddr := c.GetHeader("X-Forwarded-For")
		if clientAddr == "" {
			ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
			clientAddr = ip
		}
		c.String(http.StatusOK, clientAddr)
	})

	r.GET("/ua", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetHeader("User-Agent"))
	})

	r.GET("/headers", func(c *gin.Context) {
		var headers = Resp{}
		for headerKey, headerValue := range c.Request.Header {
			if strings.HasPrefix(headerKey, "X-Amzn") {
				continue
			}
			headers[headerKey] = strings.Join(headerValue, ",")
		}
		c.JSON(http.StatusOK, headers)
	})

	r.GET("/proto", func(c *gin.Context) {
		c.String(http.StatusOK, "Proto: %s", c.Request.Proto)
	})

	r.GET("/date", func(c *gin.Context) {
		c.String(http.StatusOK, time.Now().Format(time.RFC3339))
	})

	r.GET("/timestamp", func(c *gin.Context) {
		c.String(http.StatusOK, fmt.Sprintf("%d", time.Now().Unix()))
	})

	r.GET("/check_status", func(c *gin.Context) {
		type respType map[string]int
		var resp = respType{}
		resp["status"] = 1
		c.JSON(http.StatusOK, resp)
	})

	r.HEAD("/generate_204", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	r.GET("/generate_204", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	r.GET("/dns/:domains", func(c *gin.Context) {
		domains := c.Param("domains")
		resp := make(map[string][]string)
		if domains == "" {
			c.JSON(http.StatusOK, resp)
			return
		}
		for _, domain := range strings.Split(domains, ",") {
			ipAddrs, err := net.LookupIP(domain)
			if err != nil {
				resp[domain] = []string{fmt.Sprintf("ERR: %s", err)}
				continue
			}

			var addrs []string
			for _, ipAddr := range ipAddrs {
				if ipv4 := ipAddr.To4(); ipv4 != nil {
					addrs = append(addrs, ipv4.String())
					continue
				}
				if ipv6 := ipAddr.To16(); ipv6 != nil {
					addrs = append(addrs, ipv6.String())
				}
			}
			resp[domain] = addrs
		}
		c.JSON(http.StatusOK, resp)
	})
	r.GET("/geoip", func(c *gin.Context) {
		var target string
		addr := c.GetHeader("X-Forwarded-For")
		if addr != "" {
			ips := strings.Split(addr, ", ")
			if len(ips) >= 1 {
				target = ips[0]
			}
		}
		if target == "" {
			ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
			target = ip
		}
		result, err := geoipQuery(target, c)
		if err != nil {
			log.Println("Error: ", err)
			c.String(http.StatusBadRequest, result)
			return
		}
		result = target + "\n" + result
		c.String(http.StatusOK, result)
	})
	r.GET("/geoip/:ip", func(c *gin.Context) {
		addr := c.Param("ip")
		result, err := geoipQuery(addr, c)
		if err != nil {
			log.Println("Error: ", err)
			c.String(http.StatusBadRequest, result)
			return
		}
		c.String(http.StatusOK, result)
	})

	r.POST("/store", func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		if len(raw) < 1 {
			c.String(http.StatusBadRequest, "request empty")
			return
		}
		if len(raw) > storeSizeLimit {
			c.String(http.StatusBadRequest, "data size over limit")
			return
		}
		hash, err := storeSet(raw)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.String(http.StatusOK, "save ok, hash is %s , addr %s://%s/store/%s , expire in %d minute.", hash, c.GetHeader("X-Forwarded-Proto"), getRequestHostname(c), hash, storeExpireTime)
	})

	r.GET("/store/:hash", func(c *gin.Context) {
		hash := c.Param("hash")
		if len(hash) != storeKeyLength {
			c.String(http.StatusBadRequest, "hash format error")
			return
		}
		value, err := storeGet(hash)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		if value == nil {
			c.String(http.StatusNotFound, "hash not found")
			return
		}
		c.Header("X-Hash-SHA256", fmt.Sprintf("%x", sha256.Sum256(value)))

		//let user set content type.
		contentType := "application/octet-stream"
		output := c.Query("output")
		switch output {
		case "text", "txt", "plain":
			contentType = "text/plain"
		case "html", "htm":
			contentType = "text/html"
		case "png":
			contentType = "image/png"
		case "jpg", "jpeg":
			contentType = "image/jpeg"
		}
		c.Data(http.StatusOK, contentType, value)
	})

	r.GET("/sysinfo", func(c *gin.Context) {
		resp := make(map[string]interface{})
		resp["version"] = version
		resp["num_goroutine"] = runtime.NumGoroutine()
		resp["go_version"] = runtime.Version()
		resp["start_time"] = startTime

		//memory info
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		resp["memory"] = map[string]interface{}{
			"alloc":       printMemSize(m.Alloc),
			"total_alloc": printMemSize(m.TotalAlloc),
			"sys":         printMemSize(m.Sys),
			"num_gc":      m.NumGC,
		}

		c.JSON(http.StatusOK, resp)
	})

	return r
}

func main() {
	addr := "127.0.0.1:8000"
	if os.Getenv("PORT") != "" {
		addr = ":" + os.Getenv("PORT")
	}
	if os.Getenv("simpleHTTP") != "" {
		log.Println("listen http: ", addr)
		log.Fatal(routerEngine().Run(addr))
		return
	}
	log.Println("listen: ", addr)
	log.Fatal(gateway.ListenAndServe(addr, routerEngine()))
}

func geoipQuery(addr string, c *gin.Context) (string, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "ip parse failed", errors.New("ip parse failed: " + addr)
	}
	db, err := geoip2.Open("geoip/GeoLite2-City.mmdb")
	if err != nil {
		return "geoip db read failed", err
	}
	defer db.Close()
	// If you are using strings that may be invalid, check that ip is not nil
	record, err := db.City(ip)
	if err != nil {
		return "geoip record query failed.", err
	}

	lang := c.Query("lang")
	if lang == "" {
		lang = "en"
	}

	var result string
	//city
	result += fmt.Sprintf("City: %s\n", record.City.Names[lang])
	//Subdivisions
	if len(record.Subdivisions) > 0 {
		result += fmt.Sprintf("Subdivisions: %s\n", record.Subdivisions[0].Names[lang])
	}
	//Country
	result += fmt.Sprintf("Country: %s\n", record.Country.Names[lang])
	//Continent
	result += fmt.Sprintf("Continent: %s\n", record.Continent.Names[lang])
	result += fmt.Sprintf("ISO country code: %v\n", record.Country.IsoCode)
	result += fmt.Sprintf("Time zone: %v\n", record.Location.TimeZone)
	result += fmt.Sprintf("Coordinates: %v, %v\n", record.Location.Latitude, record.Location.Longitude)
	return result, nil
}

// Resp common response struct
type Resp map[string]string

var randomStringRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890_")
var randomStringRunesCount = len(randomStringRunes)

func generateRandomString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := make([]rune, length)
	for i := range s {
		s[i] = randomStringRunes[r.Intn(randomStringRunesCount)]
	}
	return string(s)
}

type storeItem struct {
	value    []byte
	expireAt time.Time
}

type storeCache struct {
	mu              sync.RWMutex
	data            map[string]storeItem
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

func newStoreCache(cleanupInterval time.Duration) *storeCache {
	cache := &storeCache{
		data:            make(map[string]storeItem),
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}
	go cache.cleanupLoop()
	return cache
}

func (c *storeCache) set(key string, value []byte, ttl time.Duration) {
	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.data[key] = storeItem{value: value, expireAt: expireAt}
	c.mu.Unlock()
}

func (c *storeCache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	item, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if item.expireAt.IsZero() {
		return item.value, true
	}
	if time.Now().After(item.expireAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return nil, false
	}
	return item.value, true
}

func (c *storeCache) has(key string) bool {
	c.mu.RLock()
	item, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return false
	}
	if item.expireAt.IsZero() {
		return true
	}
	return time.Now().Before(item.expireAt)
}

func (c *storeCache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopCh:
			return
		}
	}
}

func (c *storeCache) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	for key, item := range c.data {
		if !item.expireAt.IsZero() && now.After(item.expireAt) {
			delete(c.data, key)
		}
	}
	c.mu.Unlock()
}

var storeMem = newStoreCache(30 * time.Second)
var storeKeyLength = 5

func storeSet(data []byte) (string, error) {
	for i := 0; i < 8; i++ {
		key := generateRandomString(storeKeyLength)
		if storeMem.has(key) {
			continue
		}
		storeMem.set(key, data, time.Minute*time.Duration(storeExpireTime))
		return key, nil
	}
	return "", errors.New("store key generation failed")
}

func storeGet(hash string) ([]byte, error) {
	result, ok := storeMem.get(hash)
	if !ok {
		return nil, nil
	}
	return result, nil
}

func printMemSize(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}

	i := 0
	size := float64(bytes)
	for ; size >= 1024 && i < len(units)-1; i++ {
		size /= 1024
	}

	return fmt.Sprintf("%.2f %s", size, units[i])
}

func getRequestHostname(c *gin.Context) string {
	if c.GetHeader("X-Forwarded-Host") != "" {
		return c.GetHeader("X-Forwarded-Host")
	}
	return c.GetHeader("Host")
}
