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
	"strings"
	"time"
)

var certificatesAndKeysFolderName = "put_certificates_and_key_from_drawbridge_here"

// Drawbridge Protocol
// Used by Drawbridge and Emissary in TCP connections to request and route to the desired
// Protected Service application.
const (
	// Used by Emissary to tell Drawbridge which Protected Service it wants to connect to.
	// e.g PS_CONN My Minecraft Server
	ProtectedServiceConnection = "PS_CONN"
	// Used to get list of service names for use by Emissary client to tell Drawbridge what Protected Service it wants to connect to.
	ProtectedServicesList = "PS_LIST"
)

func main() {
	fmt.Println("Emissary is starting...")
	certificatesAndKeysFolderPath := utils.CreateEmissaryFileReadPath(certificatesAndKeysFolderName)

	_, err := os.Stat(certificatesAndKeysFolderPath)
	if os.IsNotExist(err) {
		runOnboarding()
	}

	// A Drawbridge admin can create an "Emissary Bundle" which will contain certificate and drawbridge server info
	// to remove the need for a user to configure Emissary manually.
	emissaryBundle := getEmissaryBundle()

	slog.Debug("Emissary is trying to read the Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt") {
		message := fmt.Sprintf("The \"emissary-mtls-tcp.crt\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please log into the Drawbridge Dashboard and click the \"Generate\" button.\n"
		message += "Once that is done, please place those files into the \"put_certificates_and_key_from_drawbridge_here\" folder and run Emissary again.\n\n"
		utils.PrintFinalError(message, nil)
	}

	slog.Debug("Emissary is trying to read the Key file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key") {
		message := fmt.Sprintf("The \"emissary-mtls-tcp.key\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please log into the Drawbridge Dashboard and click the \"Generate\" button.\n"
		message += "Once that is done, please place those files into the \"put_certificates_and_key_from_drawbridge_here\" folder and run Emissary again.\n\n"
		utils.PrintFinalError(message, nil)
	}

	slog.Debug("Emissary is trying to read the CA Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/ca.crt") {
		message := fmt.Sprintf("The \"ca.crt\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please log into the Drawbridge Dashboard and click the \"Generate\" button.\n"
		message += "Once that is done, please place those files into the \"put_certificates_and_key_from_drawbridge_here\" folder and run Emissary again.\n\n"
		utils.PrintFinalError(message, nil)
	}

	slog.Debug("Emissary is trying to load Certificate and Key file for connecting to Drawbridge...")
	// load tls configuration
	mTLSCertificatePath := utils.CreateEmissaryFileReadPath("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt")
	mTLSKeyPath := utils.CreateEmissaryFileReadPath("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key")
	cert, err := tls.LoadX509KeyPair(mTLSCertificatePath, mTLSKeyPath)
	if err != nil {
		utils.PrintFinalError("", err)
	}
	// Configure the client to trust TLS server certs issued by a CA.
	certPool, err := x509.SystemCertPool()
	if err != nil {
		utils.PrintFinalError("", err)
	}
	drawbridgeCAPath := utils.CreateEmissaryFileReadPath("./put_certificates_and_key_from_drawbridge_here/ca.crt")
	if caCertPEM, err := os.ReadFile(drawbridgeCAPath); err != nil {
		utils.PrintFinalError("", err)
	} else if ok := certPool.AppendCertsFromPEM(caCertPEM); !ok {
		utils.PrintFinalError("invalid cert in CA PEM", nil)
	}

	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
	}

	var drawbridgeAddress string
	if emissaryBundle == nil {
		fmt.Println("Please enter your Drawbridge server URL or IP (e.g drawbridge.mysite.com:3100 or 50.162.50.224:3100):")
		fmt.Println("Please note the default Drawbridge reverse proxy port is 3100.")
		fmt.Print("Drawbridge server URL or IP: ")
		fmt.Scan(&drawbridgeAddress)
		fmt.Println()
	} else {
		fmt.Printf("Connecting to Drawbridge server from local Emissary Bundle at %s...\n\n", *emissaryBundle)
		drawbridgeAddress = *emissaryBundle
	}

	serviceNames := getProtectedServiceNames(drawbridgeAddress, tlsConfig)
	runningProxies := make(map[string]net.Listener, 0)
	// TODO
	// dont run this print unless we were able to get at least one service from Drawbridge.
	fmt.Println("The following Protected Services are available:")
	port := 3200
	for i, service := range serviceNames {
		go setUpLocalSeviceProxies(service, runningProxies, drawbridgeAddress, tlsConfig, port, i)
		if err != nil {
			utils.PrintFinalError("error setting up local proxies to Drawbridge Protected Resources", err)
		}
	}
	var exitCommand string
	fmt.Scan(&exitCommand)

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
	certAndKeyFolderPath := utils.CreateEmissaryFileReadPath(certificatesAndKeysFolderName)
	if err := os.Mkdir(certAndKeyFolderPath, os.ModePerm); err != nil {
		utils.PrintFinalError("Unable to create put_certificates_and_key_from_drawbridge_here folder! This folder is required to exist, so please create it yourself or allow Emissary the permissions required to create it.", nil)
	} else {
		var noop string
		fmt.Scanln(&noop)
		os.Exit(0)
	}
}

// We need to request the list of services from Drawbridge via a TCP call.
// It doesn't _have_ to be a TCP call, but we don't need to overhead of HTTP for this, I don't think.
// And at the end of the day we need to write to our connection to Drawbridge later with the name of the service we want to connect to.
func getProtectedServiceNames(drawbridgeAddress string, tlsConfig *tls.Config) []string {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", drawbridgeAddress, tlsConfig)
	if err != nil {
		slog.Error("Failed connecting to Drawbridge mTLS TCP server", err)
		return nil
	}
	defer conn.Close()

	// Get list of Protected Services.
	conn.Write([]byte(ProtectedServicesList))
	// Read incoming data
	buf := make([]byte, 2000)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	intermediateString := strings.Split(string(buf[:]), ":")
	serviceNames := strings.Split(intermediateString[1], ",")
	return serviceNames[:len(serviceNames)-1]
}

// Since we multiplex services over the only Drawbridge port, we need a way to tell Drawbridge which service we want to connect to.
// We can do this by exposing a port locally for each service seperately, and when we connect to each proxy, we can use the
// proxy port to ma to the service name, and request to connect to that service when we are talking to Drawbridge.
func setUpLocalSeviceProxies(protectedServiceName string, localServiceProxies map[string]net.Listener, drawbridgeAddress string, tlsConfig *tls.Config, port int, i int) {
	localServiceProxyPort := port + i
	protectedServiceName = strings.TrimSpace(protectedServiceName)
	hostAndPort := fmt.Sprintf("127.0.0.1:%d", localServiceProxyPort)
	l, err := net.Listen("tcp", hostAndPort)
	if err != nil {
		utils.PrintFinalError("Emissary was unable to start the local proxy server", err)
	}
	fmt.Printf("%d) \"%s\" on port %d\n", i+1, protectedServiceName, localServiceProxyPort)

	// Save the proxy listener for use later.
	localServiceProxies[protectedServiceName] = l

	defer l.Close()
	for {
		// wait for connection from local machine
		conn, err := l.Accept()
		if err != nil {
			slog.Error("Reverse proxy TCP Accept failed", err)
		}
		// Handle new connection in a new go routine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(clientConn net.Conn) {
			slog.Info(fmt.Sprintf("TCP Accept from: %s\n", clientConn.RemoteAddr()))
			// Connect to Drawbridge .
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", drawbridgeAddress, tlsConfig)
			if err != nil {
				slog.Error("Failed connecting to Drawbridge mTLS TCP server", err)
				return
			}
			defer conn.Close()

			// Tell Drawbridge the name of the Protected Service we want to connect to.
			_, err = conn.Write([]byte(fmt.Sprintf("%s %s", ProtectedServiceConnection, protectedServiceName)))
			if err != nil {
				utils.PrintFinalError("error sending Drawbridge what Protected Service we want to connect to: %w", err)
			}

			// Copy data back and from client and server.
			go io.Copy(conn, clientConn)
			io.Copy(clientConn, conn)
			// Shut down the connection.
			clientConn.Close()
		}(conn)
	}
}

func getEmissaryBundle() *string {
	bundlePath := utils.CreateEmissaryFileReadPath("./bundle/drawbridge.txt")
	// We don't call the file path builder function here because it doesn't work.
	_, err := os.Stat(bundlePath)
	if os.IsNotExist(err) {
		return nil
	} else {
		bundleBytes := utils.ReadFile(bundlePath)
		if bundleBytes != nil {
			bundleData := strings.TrimSpace(string(*bundleBytes))
			return &bundleData
		} else {
			return nil
		}
	}
}
