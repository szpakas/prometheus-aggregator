package main

import (
	"bytes"
	"net"

	"github.com/pkg/errors"
)

type sampleHandler func(samples *sample) error

type server struct {
	sampleHandler sampleHandler
	buf           []byte
}

// newServer is factory for UDP server for incoming metrics data
//
// handler is a function of sampleHandler type responsible for dealing with incoming samples
// bs is a UDP buffer size in bytes
func newServer(handler sampleHandler, bs int) *server {
	s := server{
		sampleHandler: handler,
		buf:           make([]byte, bs),
	}
	return &s
}

func (s *server) Listen(ip string, port int) error {
	listenAddr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(ip),
	}
	conn, err := net.ListenUDP("udp", &listenAddr)
	if err != nil {
		return errors.Wrap(err, "opening server socket failed")
	}

	go func() {
		var reader *bytes.Reader
		for {
			n, _, _ := conn.ReadFromUDP(s.buf)
			reader = bytes.NewReader(s.buf[:n])

			samples, _ := parseSample(reader)

			for _, sample := range samples {
				_ = s.sampleHandler(sample)
			}
		}

		conn.Close()
	}()

	return nil
}
