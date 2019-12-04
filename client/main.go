package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/dedis/protobuf"
	core "github.com/ksei/Peerster/Core"
)

const localAddress string = "127.0.0.1"

func main() {
	args := [7]*string{}
	args[0] = flag.String("keywords", "", "Matching keywords for desired file.")
	args[1] = flag.String("budget", "", "Searching budget.")
	args[2] = flag.String("UIPort", "8080", "user interface port(for clients)")
	args[3] = flag.String("msg", "", "Message to be sent")
	args[4] = flag.String("dest", "", "destination for the private message")
	args[5] = flag.String("file", "", "file to be indexed by the gossiper")
	args[6] = flag.String("request", "", "request a chunk or metafile of this hash")

	flag.Parse()

	err := validateInput(&args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var message core.Message
	var requestBytes []byte
	requestBytes = nil
	if args[6] != nil {
		requestBytes, err = hex.DecodeString(*args[6])
		if err != nil {
			os.Exit(1)
		}
	}

	var budget *uint64
	budget = nil
	if args[1] != nil {
		i, err := strconv.ParseUint(*args[1], 10, 64)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		budget = &i
	}
	message = core.Message{Text: *args[3], Destination: args[4], File: args[5], Request: &requestBytes, KeyWords: args[0], Budget: budget}

	toSend := localAddress + ":" + *args[2]
	updAddr, err1 := net.ResolveUDPAddr("udp", toSend)
	if err1 != nil {
		fmt.Println(err1)
	}
	conn, err := net.DialUDP("udp", nil, updAddr)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()
	packetBytes, err := protobuf.Encode(&message)
	fmt.Println("Sent: ", packetBytes, message.Text)
	conn.Write(packetBytes)
}

func validateInput(args *[7]*string) error {
	argsCombination := ""
	for i, arg := range args {
		if *arg == "" {
			if i != 3 {
				args[i] = nil
			}
			argsCombination = argsCombination + "0"
			continue
		}
		argsCombination = argsCombination + "1"
	}
	combination, err := strconv.ParseInt(argsCombination, 2, 64)
	if err != nil {
		return err
	}
	allowedInputs := []int{24, 28, 18, 19, 23, 80, 112}

	for _, ai := range allowedInputs {
		if int(combination) == ai {
			return nil
		}
	}

	return errors.New("Bad argument combination")
}
