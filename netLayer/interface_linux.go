package netLayer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
)

// https://github.com/jackpal/gateway/blob/master/gateway_parsers.go
func GetGateway() (ip net.IP, ifName string, err error) {
	// See http://man7.org/linux/man-pages/man8/route.8.html
	const file = "/proc/net/route"
	var f *os.File
	f, err = os.Open(file)
	if err != nil {
		err = fmt.Errorf("Can't access %s", file)
		return
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		err = fmt.Errorf("Can't read %s", file)
		return
	}

	parsedStruct, err := parseToLinuxRouteStruct(bytes)
	if err != nil {
		return
	}

	ifName = parsedStruct.Iface

	destinationHex := "0x" + parsedStruct.Destination
	gatewayHex := "0x" + parsedStruct.Gateway

	// cast hex address to uint32
	d, err := strconv.ParseInt(gatewayHex, 0, 64)
	if err != nil {
		err = fmt.Errorf(
			"parsing default interface address field hex '%s': %w",
			destinationHex,
			err,
		)
		return
	}
	// make net.IP address from uint32
	ipd32 := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ipd32, uint32(d))

	// format net.IP to dotted ipV4 string
	ip = net.IP(ipd32)

	return
}

func parseToLinuxRouteStruct(output []byte) (linuxRouteStruct, error) {
	// parseLinuxProcNetRoute parses the route file located at /proc/net/route
	// and returns the IP address of the default gateway. The default gateway
	// is the one with Destination value of 0.0.0.0.
	//
	// The Linux route file has the following format:
	//
	// $ cat /proc/net/route
	//
	// Iface   Destination Gateway     Flags   RefCnt  Use Metric  Mask
	// eno1    00000000    C900A8C0    0003    0   0   100 00000000    0   00
	// eno1    0000A8C0    00000000    0001    0   0   100 00FFFFFF    0   00
	const (
		sep              = "\t" // field separator
		destinationField = 1    // field containing hex destination address
		gatewayField     = 2    // field containing hex gateway address
	)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	// Skip header line
	if !scanner.Scan() {
		return linuxRouteStruct{}, errors.New("Invalid linux route file")
	}

	for scanner.Scan() {
		row := scanner.Text()
		tokens := strings.Split(row, sep)
		if len(tokens) < 11 {
			return linuxRouteStruct{}, fmt.Errorf("invalid row '%s' in route file: doesn't have 11 fields", row)
		}

		// Cast hex destination address to int
		destinationHex := "0x" + tokens[destinationField]
		destination, err := strconv.ParseInt(destinationHex, 0, 64)
		if err != nil {
			return linuxRouteStruct{}, fmt.Errorf(
				"parsing destination field hex '%s' in row '%s': %w",
				destinationHex,
				row,
				err,
			)
		}

		// The default interface is the one that's 0
		if destination != 0 {
			continue
		}

		return linuxRouteStruct{
			Iface:       tokens[0],
			Destination: tokens[1],
			Gateway:     tokens[2],
			Flags:       tokens[3],
			RefCnt:      tokens[4],
			Use:         tokens[5],
			Metric:      tokens[6],
			Mask:        tokens[7],
			MTU:         tokens[8],
			Window:      tokens[9],
			IRTT:        tokens[10],
		}, nil
	}
	return linuxRouteStruct{}, errors.New("interface with default destination not found")
}

type linuxRouteStruct struct {
	Iface       string
	Destination string
	Gateway     string
	Flags       string
	RefCnt      string
	Use         string
	Metric      string
	Mask        string
	MTU         string
	Window      string
	IRTT        string
}
