package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const ReadBufferSize = 1600
const (
	HasPid uint8 = 1
	Important uint8 = 1 << 1
)

var IoDeadline, _ = time.ParseDuration("5s")
var AtRune, _ = utf8.DecodeRuneInString("@")

type FMsgAddress struct {
	User string
	Domain string
}

type FMsgAttachmentHeader struct {
	Filename string
	Size uint32
	Data []byte // TODO path instead of in memory
}

type FMsgHeader struct {
	Version uint8
	Flags uint8
	Pid [32]byte
	From FMsgAddress
	To []FMsgAddress
	Timestamp float64
	Topic string
	Type string
	Msg []byte // TODO path instead of in memory

}


func parseAddress(b []byte) (*FMsgAddress, error) {
	var addr = &FMsgAddress{}
	addrStr := string(b)
	firstAt := strings.IndexRune(addrStr, AtRune)
	if firstAt == -1 || firstAt != 0 {
		return addr, fmt.Errorf("invalid address, must start with @ %s", addr)
	}
	lastAt := strings.LastIndex(addrStr, "@")
	if lastAt == firstAt {
		return addr, fmt.Errorf("invalid address, must have second @ %s", addr)
	}
	addr.User = addrStr[1:lastAt]
	addr.Domain = addrStr[lastAt+1:]
	return addr, nil
}

// Reads byte slice prefixed with uint8 size from reader supplied
func ReadUInt8Slice(r io.Reader) ([]byte, error) {
	var size byte
	err := binary.Read(r, binary.LittleEndian, &size)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(r, int64(size)))
}

func readAddress(r io.Reader) (*FMsgAddress, error) {
	slice, err := ReadUInt8Slice(r)
	if err != nil {
		return nil, err
	}
	return parseAddress(slice)
}

func readHeader(c net.Conn) (FMsgHeader, error) {
	r := bufio.NewReaderSize(c, ReadBufferSize)
	var h FMsgHeader

	// set read deadline TODO update after reading header
	c.SetReadDeadline(time.Now().Add(IoDeadline))

	// read version
	v, err := r.ReadByte()
	if err != nil {
		return h, err
	}
	// TODO if version == 255 this is a CHALLENGE
	if v != 1 {
		return h, fmt.Errorf("unsupported version: %d", v)
	}
	h.Version = v

	// read flags
	flags, err := r.ReadByte()
	if err != nil {
		return h, err
	}
	h.Flags = flags
	
	// read pid if any
	if flags & HasPid == 1 {  
		pid, err := io.ReadAll(io.LimitReader(c, 32))
		if err != nil {
			return h, err
		}
		copy(h.Pid[:], pid)
		// TODO verify pid exists
	}

	// read from address
	from, err := readAddress(r)
	if err != nil {
		return h, err
	}
	log.Printf("from: @%s@%s", from.User, from.Domain)
	h.From = *from
 
	// read to addresses
	num, err := r.ReadByte()
	if err != nil {
		return h, err
	} 
	for num > 0 {
		addr, err := readAddress(r)
		if err != nil {
			return h, err
		}
		h.To = append(h.To, *addr)
		num--
		log.Printf("to: @%s@%s", addr.User, addr.Domain)
	}

	// read timestamp
	if err := binary.Read(r, binary.LittleEndian, &h.Timestamp); err != nil {
		return h, err
	}

	// read topic
	topic, err := ReadUInt8Slice(r)
	if err != nil {
		return h, err
	}
	h.Topic = string(topic)

	// read type
	mime, err := ReadUInt8Slice(r)
	if err != nil {
		return h, err
	}
	h.Topic = string(mime)

	return h, nil
}

func handleConn(c net.Conn) {
	log.Printf("Connection from: %s", c.RemoteAddr().String())

	// read header
	header, err := readHeader(c)
	if err != nil {
		log.Printf("Error reading header from, %s: %s", c.RemoteAddr().String(), err)
		return
	}

	resp := fmt.Sprintf("Read header from: %s, to: %s", header.From, header.To)
	resp_bytes := []byte(resp)
	c.Write(resp_bytes)
	log.Println("Voila!")

	// CHALLENGE
	// CHALLENGE RESP
	// checks
	// download

	// close
	c.Close()

}


func main() {
	ln, err := net.Listen("tcp", "localhost:36900")
	if err != nil {
		log.Fatal(err)
	}
	for {
		log.Println("Listening on localhost:36900")
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting connection from, %s: %s\n", ln.Addr().String(), err)
		} else {
			go handleConn(conn)
		}
	}
}

