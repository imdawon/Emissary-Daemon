package main

import (
	"crypto/tls"
	"crypto/x509"
	"dhes/emissary/daemon/cmd/utils"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"time"
)

var certificatesAndKeysFolderName = "put_certificates_and_key_from_drawbridge_here"

func main() {
	fmt.Println("Emissary is starting...")
	_, err := os.Stat(certificatesAndKeysFolderName)
	if os.IsNotExist(err) {
		if err := os.Mkdir(certificatesAndKeysFolderName, os.ModePerm); err != nil {
			log.Fatal("Unable to create put_certificates_and_key_from_drawbridge_here folder! This folder is required to exist, so please create it yourself or allow Emissary the permissions required to create it.")
		}
	}

	fmt.Println("Emissary is trying to read the Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt") {
		message := fmt.Sprintf(`The "emissary-mtls-tcp.crt" file is missing from the "%s" folder, which should be next to this program.
		To generate this file, please log into the Drawbridge Dashboard and click the "Generate Emissary Client and Certificate Files" button.
		Once that is done, please place those files into the "put_certificates_and_key_from_drawbridge_here" folder and run Emissary again.`, certificatesAndKeysFolderName)
		log.Fatal(message)
	}

	fmt.Println("Emissary is trying to read the Key file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key") {
		message := fmt.Sprintf(`The "emissary-mtls-tcp.key" file is missing from the "%s" folder, which should be next to this program.
		To generate this file, please log into the Drawbridge Dashboard and click the "Generate Emissary Client Certificate and Key Files" button.
		Once that is done, please place those files into the "put_certificates_and_key_from_drawbridge_here" folder and run Emissary again.`, certificatesAndKeysFolderName)
		log.Fatal(message)
	}

	fmt.Println("Emissary is trying to load Certificate and Key file for connecting to Drawbridge...")
	// load tls configuration
	cert, err := tls.LoadX509KeyPair("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt", "./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key")
	if err != nil {
		log.Fatal(err)
	}
	// Configure the client to trust TLS server certs issued by a CA.
	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Fatal(err)
	}
	if caCertPEM, err := os.ReadFile("./put_certificates_and_key_from_drawbridge_here/ca.crt"); err != nil {
		log.Fatal(err)
	} else if ok := certPool.AppendCertsFromPEM(caCertPEM); !ok {
		log.Fatal("invalid cert in CA PEM")
	}

	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
	}

	fmt.Println()
	fmt.Println("Please enter your Drawbridge server IP or URL (http://drawbridge.mysite.com:8000 or 50.162.50.224:8000):")
	var drawbridgeLocationResponse string
	fmt.Scan(&drawbridgeLocationResponse)
	fmt.Println()

	fmt.Println("The remote protected service is now available at 127.0.0.1:8008 (If everything works).")
	l, err := net.Listen("tcp", "127.0.0.1:8008")
	if err != nil {
		log.Fatalf("Emissary was unable to start the local proxy server: %s", err)
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
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", drawbridgeLocationResponse, tlsConfig)
			if err != nil {
				fmt.Errorf("Failed connecting to Drawbridge mTLS TCP server: ", err)
				return
			}
			defer conn.Close()

			slog.Debug(fmt.Sprintf("TCP Accept from: %s\n", clientConn.RemoteAddr()))
			// Copy data back and from client and server.
			go io.Copy(conn, clientConn)
			io.Copy(clientConn, conn)
			// Shut down the connection.
			clientConn.Close()
		}(conn)
	}

}
