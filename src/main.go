package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/apex/gateway"
	"github.com/gin-gonic/gin"
	"github.com/oschwald/geoip2-golang"
)

var (
	version = "debug"
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
		c.String(http.StatusOK, "version: "+version)
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
	r.GET("/geoip/:ip", func(c *gin.Context) {
		ip := net.ParseIP(c.Param("ip"))
		if ip == nil {
			c.String(http.StatusBadRequest, "ip parse failed")
			return
		}
		db, err := geoip2.Open("geoip/GeoLite2-City.mmdb")
		if err != nil {
			c.String(http.StatusGone, "geoip db read failed.")
			log.Fatal(err)
		}
		defer db.Close()
		// If you are using strings that may be invalid, check that ip is not nil
		record, err := db.City(ip)
		if err != nil {
			log.Fatal(err)
		}

		if strings.ToLower(c.Query("view")) == "json" {
			c.JSON(http.StatusOK, record)
			return
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
		c.String(http.StatusOK, result)
	})

	return r
}

func main() {
	addr := ":8000"
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

// Resp common response struct
type Resp map[string]string
