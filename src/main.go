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
	"strings"
	"time"

	"github.com/OrlovEvgeny/go-mcache"
	"github.com/apex/gateway"
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

	r.GET("/generate_204", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	r.GET("/dns/:domains", func(c *gin.Context) {
		domains := c.Param("domains")
		resp := Resp{}
		if domains == "" {
			c.JSON(http.StatusOK, resp)
			return
		}
		for _, domain := range strings.Split(domains, ",") {
			ipAddrs, err := net.LookupIP(domain)
			if err != nil {
				resp[domain] = fmt.Sprintf("ERR: %s", err)
				continue
			}

			var addr string
			for _, ipAddr := range ipAddrs {
				ipv4 := ipAddr.To4()
				if ipv4 == nil {
					continue
				}
				addr = ipv4.String()
				break
			}
			resp[domain] = addr
		}
		c.JSON(http.StatusOK, resp)
	})
	r.GET("/geoip", func(c *gin.Context) {
		addr := c.GetHeader("X-Forwarded-For")
		if addr == "" {
			ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
			addr = ip
		}
		result, err := geoipQuery(addr, c)
		if err != nil {
			log.Println("Error: ", err)
			c.String(http.StatusBadRequest, result)
			return
		}
		result = addr + "\n" + result
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
	rand.Seed(time.Now().UnixNano())
	s := make([]rune, length)
	for i := range s {
		s[i] = randomStringRunes[rand.Intn(randomStringRunesCount)]
	}
	return string(s)
}

var storeMem = mcache.New()
var storeKeyLength = 5

func storeSet(data []byte) (string, error) {
	key := generateRandomString(storeKeyLength)
	if err := storeMem.Set(key, data, time.Minute*time.Duration(storeExpireTime)); err != nil {
		return "", nil
	}
	return key, nil
}

func storeGet(hash string) ([]byte, error) {
	result, ok := storeMem.Get(hash)
	if !ok {
		return nil, nil
	}
	value, ok := result.([]byte)
	if !ok {
		return nil, nil
	}
	return value, nil
}

func printMemSize(size uint64) string {
	m := []string{"b", "kb", "mb", "gb"}
	curr := 0
	val := float64(size)
	for {
		if val < 1024 || curr >= len(m)-1 {
			return fmt.Sprintf("%.2f %s", val, m[curr])
		}
		val /= 1024
		curr++
	}
}

func getRequestHostname(c *gin.Context) string {
	if c.GetHeader("X-Forwarded-Host") != "" {
		return c.GetHeader("X-Forwarded-Host")
	}
	return c.GetHeader("Host")
}
