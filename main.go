package main

import (
	"crypto/tls"
	"crypto/x509"
	"dhes/emissary/daemon/cmd/utils"
	"fmt"
	"io"
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
		runOnboarding()
	}

	fmt.Println("Emissary is trying to read the Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt") {
		message := fmt.Sprintf("The \"emissary-mtls-tcp.crt\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please log into the Drawbridge Dashboard and click the \"Generate\" button.\n"
		message += "Once that is done, please place those files into the \"put_certificates_and_key_from_drawbridge_here\" folder and run Emissary again.\n\n"
		utils.PrintFinalError(message, nil)
	}

	fmt.Println("Emissary is trying to read the Key file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key") {
		message := fmt.Sprintf("The \"emissary-mtls-tcp.key\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please log into the Drawbridge Dashboard and click the \"Generate\" button.\n"
		message += "Once that is done, please place those files into the \"put_certificates_and_key_from_drawbridge_here\" folder and run Emissary again.\n\n"
		utils.PrintFinalError(message, nil)
	}

	fmt.Println("Emissary is trying to read the CA Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/ca.crt") {
		message := fmt.Sprintf("The \"ca.crt\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please log into the Drawbridge Dashboard and click the \"Generate\" button.\n"
		message += "Once that is done, please place those files into the \"put_certificates_and_key_from_drawbridge_here\" folder and run Emissary again.\n\n"
		utils.PrintFinalError(message, nil)
	}

	fmt.Println("Emissary is trying to load Certificate and Key file for connecting to Drawbridge...")
	// load tls configuration
	cert, err := tls.LoadX509KeyPair("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt", "./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key")
	if err != nil {
		utils.PrintFinalError("", err)
	}
	// Configure the client to trust TLS server certs issued by a CA.
	certPool, err := x509.SystemCertPool()
	if err != nil {
		utils.PrintFinalError("", err)
	}
	if caCertPEM, err := os.ReadFile("./put_certificates_and_key_from_drawbridge_here/ca.crt"); err != nil {
		utils.PrintFinalError("", err)
	} else if ok := certPool.AppendCertsFromPEM(caCertPEM); !ok {
		utils.PrintFinalError("invalid cert in CA PEM", nil)
	}

	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
	}

	fmt.Println()

	fmt.Println("Please enter your Drawbridge server URL or IP (e.g drawbridge.mysite.com:3100 or 50.162.50.224:3100):")
	fmt.Println("Please note the default Drawbridge reverse proxy port is 3100.")
	fmt.Print("Drawbridge server URL or IP: ")
	var drawbridgeLocationResponse string
	fmt.Scan(&drawbridgeLocationResponse)
	fmt.Println()

	fmt.Println("The remote protected service is now available at 127.0.0.1:4000 (If everything works).")
	l, err := net.Listen("tcp", "127.0.0.1:4000")
	if err != nil {
		utils.PrintFinalError("Emissary was unable to start the local proxy server", err)
	}

	defer l.Close()
	for {
		// wait for connection
		conn, err := l.Accept()
		if err != nil {
			slog.Error("Reverse proxy TCP Accept failed", err)
		}
		// Handle new connection in a new go routine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(clientConn net.Conn) {
			// connect to drawbridge on the port lsitening for the actual service
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", drawbridgeLocationResponse, tlsConfig)
			if err != nil {
				slog.Error("Failed connecting to Drawbridge mTLS TCP server", err)
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

func runOnboarding() {
	fmt.Println("\n* * * * * * * * * * * *")
	fmt.Println("  Welcome to Emissary!")
	fmt.Println("* * * * * * * * * * * *")
	fmt.Println("\nFIRST TIME SETUP INSTRUCTIONS:")
	fmt.Println("Please have your Drawbridge admin give you the required key and certificate files; then")
	fmt.Println("place them in the \"put_certificates_and_key_from_drawbridge_here\" folder we just created.")
	fmt.Println("Once completed, run Emissary again to connect to your Protected Service!")
	fmt.Println("\nPress Enter key to exit...")
	if err := os.Mkdir(certificatesAndKeysFolderName, os.ModePerm); err != nil {
		utils.PrintFinalError("Unable to create put_certificates_and_key_from_drawbridge_here folder! This folder is required to exist, so please create it yourself or allow Emissary the permissions required to create it.", nil)
	} else {
		var noop string
		fmt.Scanln(&noop)
		os.Exit(0)
	}
}
