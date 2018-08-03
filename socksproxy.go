package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	IP      string        `json:"ip"`
	Port    int           `json:"port"`
	Timeout time.Duration `json:"timeout"`
}

func (config *Config) GetAddr() string {
	return config.IP + ":" + strconv.Itoa(config.Port)
}

func (config *Config) GetTimeout() time.Duration {
	return config.Timeout * time.Second
}

func ReadConfig(path string) (config *Config, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}
	config = &Config{}
	if err = json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return
}

func main() {
	var config *Config

	if len(os.Args) == 1 {
		config = &Config{
			IP:      "0.0.0.0",
			Port:    8080,
			Timeout: 8,
		}
	} else {
		var err error
		config, err = ReadConfig(os.Args[1])
		if err != nil {
			log.Fatalf("read config [%s] failed: %v\n", os.Args[1], err)
		}
	}

	listener, err := net.Listen("tcp", config.GetAddr())
	if err != nil {
		log.Fatalf("listen failed: %v\n", err)
	}
	log.Println("listen at: " + config.GetAddr())
	log.Printf("timeout: %s", config.GetTimeout())

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Fatalf("accept failed: %v\n", err)
		}
		go handleConn(client, config.GetTimeout())
	}
}

func handleConn(client net.Conn, timeout time.Duration) {
	defer client.Close()

	var buf [1024]byte
	n, err := client.Read(buf[:])
	if err != nil {
		log.Printf("read error: %v\n", err)
		return
	}
	if n < 3 {
		log.Println("invalid auth request")
		return
	}
	if buf[0] != 0x5 {
		log.Printf("unsupport version: %d\n", buf[0])
		return
	}

	client.Write([]byte{0x5, 0x0})

	n, err = client.Read(buf[:])
	if err != nil {
		log.Printf("read error: %v\n", err)
		return
	}
	if n < 7 {
		log.Println("invalid command request")
		return
	}
	if buf[1] != 0x1 {
		log.Printf("unsupport command: %d\n", buf[1])
		return
	}

	var host string
	switch buf[3] {
	case 0x1: // ipv4
		host = net.IPv4(buf[4], buf[5], buf[6], buf[7]).String()
	case 0x3: // domain
		host = string(buf[5 : 5+buf[4]])
	case 0x4: // ipv6
		host = net.IP{
			buf[4], buf[5], buf[6], buf[7],
			buf[8], buf[9], buf[10], buf[11],
			buf[12], buf[13], buf[14], buf[15],
			buf[16], buf[17], buf[18], buf[19]}.String()
	}
	port := strconv.Itoa(int(buf[n-2])<<8 | int(buf[n-1]))
	addr := net.JoinHostPort(host, port)

	server, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		log.Printf("dail to [%s] failed: %v\n", addr, err)
		return
	}
	defer server.Close()

	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	go io.Copy(client, server)
	io.Copy(server, client)
}
