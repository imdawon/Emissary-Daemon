package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/slog"
	"net"
	"time"
)

func main() {
	// load tls configuration
	cert, err := tls.LoadX509KeyPair("../Drawbridge/cmd/reverse_proxy/ca/server-cert.crt", "../Drawbridge/cmd/reverse_proxy/ca/server-key.key")
	if err != nil {
		panic(err)
	}
	// Configure the client to trust TLS server certs issued by a CA.
	certPool, err := x509.SystemCertPool()
	if err != nil {
		panic(err)
	}
	if caCertPEM, err := ioutil.ReadFile("../Drawbridge/cmd/reverse_proxy/ca/ca.crt"); err != nil {
		panic(err)
	} else if ok := certPool.AppendCertsFromPEM(caCertPEM); !ok {
		panic("invalid cert in CA PEM")
	}

	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
	}

	fmt.Println("Emissary Local Proxy Server listening at 127.0.0.1:25565")
	l, err := net.Listen("tcp", "127.0.0.1:25565")
	if err != nil {
		slog.Error(fmt.Sprintf("TCP Listen failed: %s", err))
	}

	defer l.Close()
	for {
		// wait for connection
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("Reverse proxy TCP Accept failed: %s", err)
		}
		// Handle new connection in a new go routine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(clientConn net.Conn) {
			// connect to drawbridge on the port lsitening for the actual service
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", "localhost:25567", tlsConfig)
			if err != nil {
				fmt.Println("Failed connecting to mTLS server: ", err)
				return
			}
			defer conn.Close()

			slog.Info(fmt.Sprintf("TCP Accept from: %s\n", clientConn.RemoteAddr()))
			// Copy data back and from client and server.
			go io.Copy(conn, clientConn)
			io.Copy(clientConn, conn)
			// Shut down the connection.
			clientConn.Close()
		}(conn)
	}

}
