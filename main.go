package main

import (
	"log"
	"net"
	"track_proxy/api_handler"
	"track_proxy/connection_handler"
	"track_proxy/tls_utils"
	"track_proxy/web_app"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	tls "github.com/refraction-networking/utls"
)

func listenProxy(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("Error during creation of TCP listener ", err)
	}
	defer listener.Close()

	log.Println("Starting proxy server on addr", addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection %v \n", err)
			conn.Close()
		}

		log.Printf("Processing connection %v \n", conn)
		go func() {
			success := connection_handler.HandleConnection(conn)
			log.Printf("Connecion %v success status: %v \n", conn.RemoteAddr().String(), success)
		}()
	}
}

func listenProxyTls(addr string) {
	listener, err := tls.Listen("tcp", addr, tls_utils.TLSConfig)
	if err != nil {
		log.Fatalln("Error during creation of TCP listener ", err)
	}
	defer listener.Close()

	log.Println("Starting proxy server on addr", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection %v \n", err)
		}

		log.Printf("Processing connection %v \n", conn)
		go func() {
			success := connection_handler.HandleConnection(conn)
			log.Printf("Connecion %v success status: %v \n", conn.RemoteAddr().String(), success)
		}()
	}
}

func listenApiServer(addr string) {
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8002"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Referer", "hx-current-url", "hx-request", "hx-target"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	router.GET("/ping", api_handler.Ping)
	router.GET("/requests", api_handler.GetRequests)
	router.GET("/request/:requestId", api_handler.GetRequestById)
	router.GET("/curl/:requestId", api_handler.GetCurl)
	router.DELETE("/clear", api_handler.Clear)

	log.Println("Starting gin server on addr", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalln("Error starting gin server:", err)
	}
}

func listenWebApp(addr string) {
	// http.HandleFunc("/", web_app.HandleIndex)
	// http.HandleFunc("/requests_table", web_app.HandleRequestsTable)
	// http.HandleFunc("/filter_requests", web_app.HandleFilterRequests)
	// log.Println("Starting web app server on addr", addr)
	// if err := http.ListenAndServe(addr, nil); err != nil {
	// 	log.Fatalln("Error starting web app:", err)
	// }

	router := gin.Default()
	router.SetFuncMap(web_app.FuncMap)

	// router.LoadHTMLGlob("templates/*")
	// router.GET("/", web_app.HandleIndex)

	router.GET("/", func(c *gin.Context) {
		router.LoadHTMLGlob("templates/*")
		web_app.HandleIndex(c)
	})

	router.GET("/requests_table", func(c *gin.Context) {
		router.LoadHTMLGlob("templates/*")
		web_app.HandleRequestsTable(c)
	})

	router.GET("/request-detail/:requestId", func(c *gin.Context) {
		router.LoadHTMLGlob("templates/*")
		web_app.HandleRequestsTable(c)
	})

	router.POST("/filter_requests", func(c *gin.Context) {
		router.LoadHTMLGlob("templates/*")
		web_app.HandleFilterRequests(c)
	})

	router.GET("/register-request/:requestId", web_app.RegisterActiveRequest)
	router.GET("/unregister-request", web_app.UnregisterActiveRequest)
	router.GET("/curl", web_app.GetCurl)
	router.GET("/register-request-detail/:detailType", web_app.RegisterRequestDetailType)
	log.Println("Starting web app server on addr", addr)

	log.Println("Starting gin server on addr", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalln("Error starting gin server:", err)
	}
}

func main() {
	go listenProxy(":8000")
	go listenProxyTls(":8001")
	go listenApiServer(":8002")
	go listenWebApp(":8003")

	select {}
}
