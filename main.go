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
	"sync"
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

	// A Drawbridge admin should create an "Emissary Bundle" which will contain certificate and drawbridge server info
	// to remove the need for a user to configure Emissary manually.
	emissaryBundle := getDrawbridgeNetworkAddress()

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
	// Load the TLS configuration from disk.
	mTLSCertificatePath := utils.CreateEmissaryFileReadPath("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.crt")
	mTLSKeyPath := utils.CreateEmissaryFileReadPath("./put_certificates_and_key_from_drawbridge_here/emissary-mtls-tcp.key")
	cert, err := tls.LoadX509KeyPair(mTLSCertificatePath, mTLSKeyPath)
	if err != nil {
		utils.PrintFinalError("", err)
	}
	// Configure the Emissary client to trust TLS server certs issued by the Drawbridge CA.
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
		runOutboundProxy(*outboundService, *serviceName, drawbridgeAddress, tlsConfig)
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

func runOutboundProxy(localService, serviceName, drawbridgeAddress string, tlsConfig *tls.Config) error {
	for {
		// Connect to Drawbridge.
		drawbridgeConn, err := establishConnectionToDrawbridge(drawbridgeAddress, tlsConfig)
		if err != nil {
			slog.Error("Failed to connect to Drawbridge, retrying in 5 seconds", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Register the specified Emissary Outbound service with Drawbridge.
		registerMsg := fmt.Sprintf("%s %s", OutboundConnectionCreate, serviceName)
		if _, err := drawbridgeConn.Write([]byte(registerMsg)); err != nil {
			slog.Error("Failed to register service with Drawbridge", "error", err)
			drawbridgeConn.Close()
			continue
		}

		// Wait for message acknowledgement by Drawbridge server.
		buf := make([]byte, 1024)
		n, err := drawbridgeConn.Read(buf)
		if err != nil || string(buf[:n]) != "ACK" {
			slog.Error("Failed to receive acknowledgement from Drawbridge", "error", err)
			drawbridgeConn.Close()
			continue
		}

		slog.Info("Registered outbound service", "service", serviceName, "localAddress", localService)

		handleOutboundConnection(drawbridgeConn, localService)

		// Close the Drawbridge connection after handling.
		drawbridgeConn.Close()
	}
}

// Note: This is referred to as the "Emissary Outbound" feature in the docs.
// Handle proxying traffic from a locally running networked service and the Drawbridge server.
// This exposes the locally running program to a Drawbridge server, without having to run Drawbridge on the same machine as Emissary.
func handleOutboundConnection(drawbridgeConn net.Conn, localService string) {
	defer drawbridgeConn.Close()

	// Connect to the local service we will expose as a Protected Service.
	localConn, err := net.Dial("tcp", localService)
	if err != nil {
		slog.Error("Failed to connect to local service", "error", err)
		return
	}
	defer localConn.Close()

	proxyData(drawbridgeConn, localConn)

	slog.Info("Connection closed")
}

// This function should be called when we detect that the Emissary Bundle files are missing e.g the
// certificates and pubkey keypair from the put_certificates_and_keys_here folder created during onboarding or when
// an admin creates an Emissary Bundle zip file for a user.
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
	conn, err := establishConnectionToDrawbridge(drawbridgeAddress, tlsConfig)
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

// Since we multiplex services over the single Drawbridge port, we need a way to tell Drawbridge which service we want to connect to.
// We can do this by exposing a port locally for each service seperately, and when we connect to each proxy, we can use the
// proxy port to map to the service name, and request to connect to that service when we are talking to Drawbridge.
func setUpLocalSeviceProxies(protectedServiceString string, localServiceProxies map[string]net.Listener, drawbridgeAddress string, tlsConfig *tls.Config, port int) {
	protectedServiceString = strings.TrimSpace(protectedServiceString)
	// This is the id of the service in Drawbridge.
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
		clientConn, err := l.Accept()
		if err != nil {
			slog.Error("Reverse proxy TCP Accept failed", err)
			continue
		}

		go handleConnection(clientConn, drawbridgeAddress, tlsConfig, protectedServiceString)
	}
}

// This is the function that handles forwarding and receiving traffic to and from Drawbridge and another source.
// This is used for both regular connections to Drawbridge Protected Services and when exposing a networked application running
// on the same machine as Emissary; AKA the "Emissary Outbound" feature.
func proxyData(dst net.Conn, src net.Conn) {
	defer dst.Close()
	defer src.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := io.Copy(dst, src)
		if err != nil {
			slog.Error("Failed to copy src to dst", "error", err)
		}
		dst.Close()

	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(src, dst)
		if err != nil {
			slog.Error("Failed to copy dst to src", "error", err)
		}
		src.Close()
	}()

	wg.Wait()
}

// Handle a regular (not Emissary Outbbound) connection between the Emissary client and the Drawbridge server.
func handleConnection(clientConn net.Conn, drawbridgeAddress string, tlsConfig *tls.Config, protectedServiceString string) {
	defer clientConn.Close()

	drawbridgeConn, err := establishConnectionToDrawbridge(drawbridgeAddress, tlsConfig)
	if err != nil {
		slog.Error("Failed to connect to Drawbridge", "error", err)
		return
	}
	defer drawbridgeConn.Close()

	// Tell Drawbridge the name of the Protected Service we want to connect to.
	protectedServiceConnectionMessage := fmt.Sprintf("%s %s", ProtectedServiceConnection, protectedServiceString)
	_, err = drawbridgeConn.Write([]byte(protectedServiceConnectionMessage))
	if err != nil {
		slog.Error("Error sending Protected Service connection message", "error", err)
		return
	}

	proxyData(drawbridgeConn, clientConn)
}

func establishConnectionToDrawbridge(drawbridgeAddress string, tlsConfig *tls.Config) (net.Conn, error) {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", drawbridgeAddress, tlsConfig)
	if err != nil {
		slog.Error("Failed connecting to Drawbridge mTLS TCP server", err)
		return nil, err
	}
	slog.Info("Connected to Drawbridge!")
	return conn, nil

}

func getDrawbridgeNetworkAddress() *string {
	bundleBytes := utils.ReadFile("./bundle/drawbridge.txt")
	if bundleBytes != nil {
		bundleData := strings.TrimSpace(string(*bundleBytes))
		return &bundleData
	} else {
		return nil
	}
}
