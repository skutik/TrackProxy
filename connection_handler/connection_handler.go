package connection_handler

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"track_proxy/cert_handler"
	"track_proxy/requests_storage"
	"track_proxy/tls_utils"

	tls "github.com/refraction-networking/utls"
)

const BufferSize = 1024 * 4

const OK_RESPONSE = "HTTP/1.1 200 Connection Established\r\n\r\n"
const HOST_TIMEOUT = time.Second * 30
const DEFAULT_PROTO = "http/1.1"

type ClientHelloUtlsConn struct {
	*tls.Conn
	ClientHelloRaw []byte
}

func Server(conn net.Conn, config *tls.Config) *ClientHelloUtlsConn {
	tlsConn := tls.Server(conn, config)
	return &ClientHelloUtlsConn{Conn: tlsConn}
}

func (c *ClientHelloUtlsConn) Read(b []byte) (int, error) {
	if len(c.ClientHelloRaw) == 0 {
		// Capture the first read, which should include the ClientHello
		n, err := c.Conn.Read(b)
		if err != nil {
			return n, err
		}
		c.ClientHelloRaw = append(c.ClientHelloRaw, b[:n]...)
		return n, err
	}
	return c.Conn.Read(b)
}

func parseHost(data []byte) string {
	strData := string(data)
	parts := strings.Split(strData, "\n")
	connectInfo := strings.TrimSpace(parts[0])
	parts = strings.Split(connectInfo, " ")
	return parts[1]
}

func handleConnectRequest(conn net.Conn, req *http.Request, errChan chan error) {
	host := req.Host
	logger := log.New(
		log.Writer(),
		fmt.Sprintf("[handleConnectRequest - %s] ", host),
		log.Flags(),
	)
	logger.Println("connection to host", host)
	proxyInfo := requests_storage.ProxyInfo{
		Host:    host,
		SrcAddr: conn.RemoteAddr().String(),
		DstAddr: conn.LocalAddr().String(),
	}
	request := requests_storage.Storage.CreateRecordWithProxyInfo(proxyInfo)

	var hostDomain string
	var err error
	if strings.Contains(host, ":") {
		hostDomain, _, err = net.SplitHostPort(host)
		if err != nil {
			errChan <- err
			return
		}
	} else {
		hostDomain = host
	}
	pemCert, pemKey := cert_handler.CreateCert(
		hostDomain, tls_utils.ServerCert.File, tls_utils.ServerCert.Key, 240,
	)

	tlsCert, err := tls.X509KeyPair(pemCert, pemKey)
	if err != nil {
		errChan <- err
		return
	}

	_, err = conn.Write([]byte(OK_RESPONSE))
	if err != nil {
		errChan <- fmt.Errorf("error when writing to conn %v", err)
		return
	}

	hostConn, err := tls.Dial("tcp", host, &tls.Config{
		NextProtos: []string{"h2", "http/1.1"},
	})
	if err != nil {
		logger.Printf("Error connecting to %s: %v", host, err)
		errChan <- err
		return
	}

	serverProto := hostConn.ConnectionState().NegotiatedProtocol
	if serverProto == "" {
		serverProto = DEFAULT_PROTO
	}

	logger.Printf("server proto: %s, host: %s", serverProto, host)
	// var tlsConn *ClientHelloUtlsConn
	// tlsConn := &ClientHelloUtlsConn{Conn: conn.(*tls.Conn)}

	// newConn := tls_utils.UpgradeClientConn(conn, tlsCert, serverProto)

	// var tlsServerConn *tls.Conn
	serverTlsConfig := &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256},
		MinVersion:               tls.VersionTLS13,
		Certificates:             []tls.Certificate{tlsCert},
		NextProtos:               []string{serverProto},
		InsecureSkipVerify:       true,
	}

	tlsServerConn := tls.Server(conn, serverTlsConfig)
	defer func() {
		tlsServerConn.Close()
		logger.Println("closing TLS server conn")
	}()

	if err != nil {
		errChan <- fmt.Errorf("error when creating connection to %v", host)
		return
	}

	defer func() {
		logger.Println("Closing connection to host", host)
		defer hostConn.Close()
	}()

	logger.Println("Starting pipe")
	requestChan := make(chan requests_storage.Request)

	var wg sync.WaitGroup

	wg.Add(1)
	go PipeHttp(tlsServerConn, hostConn, &wg, requestChan)
	request = <-requestChan

	err = requests_storage.Storage.AddRequestToStorage(request)
	if err != nil {
		log.Println("error when adding request to storage")
	}
	close(requestChan)
	wg.Wait()
}

func handlerDirectRequest(conn net.Conn, req *http.Request, errChan chan error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req.RequestURI = ""
	res, err := client.Do(req)
	if err != nil {
		errChan <- fmt.Errorf("error processing http request: %s", err)
		return
	}
	err = res.Write(conn)
	if err != nil {
		errChan <- fmt.Errorf("error writing to client connection %s", err)
		return
	}
	errChan <- nil
}

func HandleConnection(conn net.Conn) bool {
	defer func() {
		conn.Close()
		log.Println("Closing client connection", conn)
	}()

	// Set read deadline to prevent hanging
	if err := conn.SetReadDeadline(time.Now().Add(HOST_TIMEOUT)); err != nil {
		log.Printf("Error setting read deadline: %v", err)
		return false
	}

	var cpBuffer bytes.Buffer
	teeReader := io.TeeReader(conn, &cpBuffer)
	connReader := bufio.NewReader(teeReader)
	req, err := http.ReadRequest(connReader)
	if err != nil {
		fmt.Println("Error when reading request", err)
		return false
	}

	log.Printf("Request: %s %s (conn: %v, %s)", req.Method, req.URL, conn, conn.RemoteAddr())

	errChan := make(chan error)
	if req.Method == http.MethodConnect {
		go handleConnectRequest(conn, req, errChan)
	} else {
		log.Printf("Direct request method: %s", req.Method)
		go handlerDirectRequest(conn, req, errChan)
	}

	// Reset read deadline for subsequent operations
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("Error clearing read deadline: %v", err)
		return false
	}

	err = <-errChan
	if err != nil {
		log.Println("error processing connection to host", err)
		return false
	}
	return true
}
