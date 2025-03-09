package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

type UserAgent struct {
	Product  string `json:"product"`
	Version  string `json:"version"`
	RawValue string `json:"raw_value"`
}

type IPInfo struct {
	IP         string    `json:"ip"`
	IPDecimal  uint32    `json:"ip_decimal"`
	Country    string    `json:"country"`
	CountryISO string    `json:"country_iso"`
	CountryEU  bool      `json:"country_eu"`
	RegionName string    `json:"region_name"`
	RegionCode string    `json:"region_code"`
	City       string    `json:"city"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	TimeZone   string    `json:"time_zone"`
	ASN        string    `json:"asn"`
	ASNOrg     string    `json:"asn_org"`
	UserAgent  UserAgent `json:"user_agent"`
}

func getIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	} else {
		ip = strings.Split(ip, ",")[0]
	}

	if ip == "127.0.0.1" || ip == "::1" {
		ip = getPublicIP()
	}
	return ip
}

func getPublicIP() string {
	resp, err := http.Get("https://api64.ipify.org?format=text")
	if err != nil {
		fmt.Println("Lỗi khi lấy IP public:", err)
		return "UNKNOWN"
	}
	defer resp.Body.Close()

	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	return strings.TrimSpace(string(buf[:n]))
}

func getGeoInfo(ip string) (IPInfo, error) {
	url := "http://ip-api.com/json/" + ip + "?fields=status,message,query,country,countryCode,regionName,region,city,lat,lon,timezone,as"
	resp, err := http.Get(url)
	if err != nil {
		return IPInfo{}, err
	}
	defer resp.Body.Close()

	var data struct {
		Status     string  `json:"status"`
		Message    string  `json:"message"`
		IP         string  `json:"query"`
		Country    string  `json:"country"`
		CountryISO string  `json:"countryCode"`
		RegionName string  `json:"regionName"`
		RegionCode string  `json:"region"`
		City       string  `json:"city"`
		Latitude   float64 `json:"lat"`
		Longitude  float64 `json:"lon"`
		TimeZone   string  `json:"timezone"`
		ASN        string  `json:"as"`
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return IPInfo{}, err
	}

	if data.Status != "success" {
		return IPInfo{}, fmt.Errorf("API lỗi: %s", data.Message)
	}

	asnParts := strings.SplitN(data.ASN, " ", 2)
	asnNumber := asnParts[0]
	asnOrg := ""
	if len(asnParts) > 1 {
		asnOrg = asnParts[1]
	}

	return IPInfo{
		IP:         data.IP,
		IPDecimal:  ipToDecimal(data.IP),
		Country:    data.Country,
		CountryISO: data.CountryISO,
		CountryEU:  isEU(data.CountryISO),
		RegionName: data.RegionName,
		RegionCode: data.RegionCode,
		City:       data.City,
		Latitude:   data.Latitude,
		Longitude:  data.Longitude,
		TimeZone:   data.TimeZone,
		ASN:        asnNumber,
		ASNOrg:     asnOrg,
	}, nil
}

func ipToDecimal(ip string) uint32 {
	parts := strings.Split(ip, ".")
	var decimal uint32
	for _, p := range parts {
		decimal = decimal<<8 + uint32(parseInt(p))
	}
	return decimal
}

func isEU(countryISO string) bool {
	euCountries := map[string]bool{
		"AT": true, "BE": true, "BG": true, "HR": true, "CY": true,
		"CZ": true, "DK": true, "EE": true, "FI": true, "FR": true,
		"DE": true, "GR": true, "HU": true, "IE": true, "IT": true,
		"LV": true, "LT": true, "LU": true, "MT": true, "NL": true,
		"PL": true, "PT": true, "RO": true, "SK": true, "SI": true,
		"ES": true, "SE": true,
	}
	return euCountries[countryISO]
}

func parseInt(s string) int {
	var num int
	fmt.Sscanf(s, "%d", &num)
	return num
}

func handler(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	userAgent := r.UserAgent()

	geoInfo, err := getGeoInfo(ip)
	if err != nil {
		http.Error(w, "Lỗi khi lấy thông tin địa lý", http.StatusInternalServerError)
		fmt.Println("Lỗi lấy thông tin IP:", err)
		return
	}

	uaParts := strings.SplitN(userAgent, "/", 2)
	product, version := "Unknown", "Unknown"
	if len(uaParts) == 2 {
		product, version = uaParts[0], uaParts[1]
	}

	geoInfo.UserAgent = UserAgent{
		Product:  product,
		Version:  version,
		RawValue: userAgent,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(geoInfo)
}

func main() {
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	http.HandleFunc("/", handler)
	fmt.Println("Server đang chạy tại http://localhost:" + port)
	http.ListenAndServe(":"+port, nil)
}
