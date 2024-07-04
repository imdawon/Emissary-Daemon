package main

import (
	"crypto/tls"
	"crypto/x509"
	"dhes/emissary/daemon/cmd/utils"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	outboundService = flag.String("outbound", "", "Local service to proxy (e.g., localhost:5432)")
	serviceName     = flag.String("service-name", "", "Name of the service to register with Drawbridge")
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

const (
	OutboundConnectionCreate = "OB_CR8T"
	OutboundConnection       = "OB_CONN"
)

func main() {
	flag.Parse()
	fmt.Println("Emissary is starting...")
	certificatesAndKeysFolderPath := utils.CreateEmissaryFileReadPath(certificatesAndKeysFolderName)

	_, err := os.Stat(certificatesAndKeysFolderPath)
	if os.IsNotExist(err) {
		runOnboarding()
	}

	// A Drawbridge admin can create an "Emissary Bundle" which will contain certificate and drawbridge server info
	// to remove the need for a user to configure Emissary manually.
	emissaryBundle := getDrawbridgeAddress()

	slog.Debug("Emissary is trying to read the Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt") {
		message := fmt.Sprintf("The \"emissary-mtls-tcp.crt\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please request an Emissary Bundle from your Drawbridge admin.\n"
		utils.PrintFinalError(message, nil)
	}

	slog.Debug("Emissary is trying to read the Key file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key") {
		message := fmt.Sprintf("The \"emissary-mtls-tcp.key\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please request an Emissary Bundle from your Drawbridge admin.\n"
		utils.PrintFinalError(message, nil)
	}

	slog.Debug("Emissary is trying to read the CA Certificate file...")
	if !utils.FileExists("./put_certificates_and_key_from_drawbridge_here/ca.crt") {
		message := fmt.Sprintf("The \"ca.crt\" file is missing from the \"%s\" folder, which should be next to this program.\n", certificatesAndKeysFolderName)
		message += "To generate this file, please request an Emissary Bundle from your Drawbridge admin.\n"
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

	if *outboundService != "" {
		slog.Info("Attempting Outbound Mode...")
		if *serviceName == "" {
			utils.PrintFinalError("Service name must be provided when using outbound mode", nil)
		}
		setupOutboundProxy(*outboundService, *serviceName, drawbridgeAddress, tlsConfig)
		utils.PrintFinalError("Outbound Proxy Failed", fmt.Errorf("connection to Drawbridge has ceased"))
	}

	serviceNames := getProtectedServiceNames(drawbridgeAddress, tlsConfig)
	runningProxies := make(map[string]net.Listener, 0)
	// TODO
	// dont run this print unless we were able to get at least one service from Drawbridge.
	fmt.Println("The following Protected Services are available:")
	port := 3200
	for _, service := range serviceNames {
		go setUpLocalSeviceProxies(service, runningProxies, drawbridgeAddress, tlsConfig, port)
		if err != nil {
			utils.PrintFinalError("error setting up local proxies to Drawbridge Protected Resources", err)
		}
	}

	var exitCommand string
	fmt.Scan(&exitCommand)

}

func setupOutboundProxy(localService, serviceName, drawbridgeAddress string, tlsConfig *tls.Config) {
	conn, err := establishConnection(drawbridgeAddress, tlsConfig)
	if err != nil {
		utils.PrintFinalError("Failed to connect to Drawbridge", err)
	}
	defer conn.Close()

	// Register the service with Drawbridge
	registerMsg := fmt.Sprintf("%s %s", OutboundConnectionCreate, serviceName)
	if _, err := conn.Write([]byte(registerMsg)); err != nil {
		utils.PrintFinalError("Failed to register service with Drawbridge", err)
	}

	slog.Info("Waiting for ack from Drawbridge...")
	// Read acknowledgement from Drawbridge
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || string(buf[:n]) != "ACK" {
		utils.PrintFinalError("Failed to receive acknowledgement from Drawbridge", err)
	}

	slog.Info(fmt.Sprintf("Registered outbound service %s for %s", serviceName, localService))

	handleDrawbridgeConnections(conn, localService)
}

func handleDrawbridgeConnections(drawbridgeConn net.Conn, localService string) {
	defer drawbridgeConn.Close()

	buffer := make([]byte, 4096)
	for {
		// Read the request from Drawbridge
		n, err := drawbridgeConn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				slog.Error("Error reading from Drawbridge", "error", err)
			}
			return
		}

		// Connect to local service for each request
		localConn, err := net.Dial("tcp", localService)
		if err != nil {
			slog.Error("Failed to connect to local service", "error", err)
			continue
		}

		// Send request to local service
		_, err = localConn.Write(buffer[:n])
		if err != nil {
			slog.Error("Error writing to local service", "error", err)
			localConn.Close()
			continue
		}

		// Read response from local service
		var responseData []byte
		responseBuffer := make([]byte, 4096)
		for {
			n, err := localConn.Read(responseBuffer)
			if err != nil {
				if err != io.EOF {
					slog.Error("Error reading from local service", "error", err)
				}
				break
			}
			responseData = append(responseData, responseBuffer[:n]...)
			if n < len(responseBuffer) {
				break
			}
		}

		// Send response back to Drawbridge
		_, err = drawbridgeConn.Write(responseData)
		if err != nil {
			slog.Error("Error writing response to Drawbridge", "error", err)
			return
		}

		localConn.Close()
	}
}
func runOnboarding() {
	fmt.Println("\n* * * * * * * * * * * *")
	fmt.Println("  Welcome to Emissary!")
	fmt.Println("* * * * * * * * * * * *")
	fmt.Println("\nFIRST TIME SETUP INSTRUCTIONS:")
	fmt.Println("If you're seeing this, you aren't using an Emissary Bundle or deleted your bundle and put_certificates_and_keys_here folder and files.")
	utils.PrintFinalError("Reach out to your Drawbridge admin and ask for one :)", nil)
}

// We need to request the list of services from Drawbridge via a TCP call.
// It doesn't _have_ to be a TCP call, but we don't need to overhead of HTTP for this, I don't think.
// And at the end of the day we need to write to our connection to Drawbridge later with the name of the service we want to connect to.
func getProtectedServiceNames(drawbridgeAddress string, tlsConfig *tls.Config) []string {
	conn, err := establishConnection(drawbridgeAddress, tlsConfig)
	if err != nil {
		slog.Error("Drawbridge Connection Failed - Retrying in 5 seconds")
		fiveSecondsFromNow := time.Until(time.Now().Add(time.Second * 5))
		time.AfterFunc(fiveSecondsFromNow, func() {
			getProtectedServiceNames(drawbridgeAddress, tlsConfig)
		})
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
func setUpLocalSeviceProxies(protectedServiceString string, localServiceProxies map[string]net.Listener, drawbridgeAddress string, tlsConfig *tls.Config, port int) {
	protectedServiceString = strings.TrimSpace(protectedServiceString)
	// This is the id of the service in Drawbridge.
	// It is used
	portOffset, err := strconv.Atoi(protectedServiceString[1:3])
	protectedServiceName := protectedServiceString[3:]
	if err != nil {
		utils.PrintFinalError("Error parsing protected service string: %w", err)
	}
	localServiceProxyPort := port + portOffset
	hostAndPort := fmt.Sprintf("127.0.0.1:%d", localServiceProxyPort)
	l, err := net.Listen("tcp", hostAndPort)
	if err != nil {
		utils.PrintFinalError("Emissary was unable to start the local proxy server", err)
	}
	fmt.Printf("• \"%s\" on localhost:%d\n", protectedServiceName, localServiceProxyPort)

	// Save the proxy listener for use later.
	localServiceProxies[protectedServiceString] = l

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
			var conn net.Conn
			const maxRetries = 999
			retries := 0
			for {
				conn, err = establishConnection(drawbridgeAddress, tlsConfig)
				if err == nil {
					// Connection established successfully, handle it
					break
				}

				retries++
				if retries >= maxRetries {
					// Maximum retries reached, handle the error
					slog.Error("Failed to establish connection after", maxRetries, "retries")
					return
				}

				// Wait for a short duration before retrying
				slog.Error("Failed to establish connection to Drawbridge. Retrying in 1 second...")
				time.Sleep(1 * time.Second)
			}

			defer conn.Close()

			// Tell Drawbridge the name of the Protected Service we want to connect to.
			protectedServiceConnectionMessage := fmt.Sprintf("%s %s", ProtectedServiceConnection, protectedServiceString)
			_, err = conn.Write([]byte(protectedServiceConnectionMessage))
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

func establishConnection(drawbridgeAddress string, tlsConfig *tls.Config) (net.Conn, error) {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", drawbridgeAddress, tlsConfig)
	if err != nil {
		slog.Error("Failed connecting to Drawbridge mTLS TCP server", err)
		return nil, err
	}
	slog.Info("Connected to Drawbridge!")
	return conn, nil

}

func getDrawbridgeAddress() *string {
	bundleBytes := utils.ReadFile("./bundle/drawbridge.txt")
	if bundleBytes != nil {
		bundleData := strings.TrimSpace(string(*bundleBytes))
		return &bundleData
	} else {
		return nil
	}
}
