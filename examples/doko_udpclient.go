package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	udpServer, err := net.ResolveUDPAddr("udp", "127.0.0.1:63782")

	if err != nil {
		println("ResolveUDPAddr failed:", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		println("Listen failed:", err.Error())
		os.Exit(1)
	}

	fmt.Println("ready to input")

	defer conn.Close()
	var str string
	for {
		_, err = fmt.Scanln(&str)
		if err != nil {
			println("Scanln failed:", err.Error())
			os.Exit(1)
		}
		conn.Write([]byte(str))
	}

}
