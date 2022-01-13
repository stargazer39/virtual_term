package main

import (
	"log"
	"net/http"
	"stargazer/virtual_term/modal"

	"crypto/rand"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Server struct {
	s    *gin.Engine
	tman *TermManager
}

type TerminalParams struct {
	Title  string   `json:"title"`
	Params []string `json:"params"`
}

func NewServer() (*Server, error) {
	s := gin.Default()
	bytes, gErr := GenRandomBytes(32)

	if gErr != nil {
		return nil, gErr
	}

	store := cookie.NewStore(bytes)
	s.Use(sessions.Sessions("defaultsession", store))

	return &Server{
		s:    s,
		tman: NewTermManager(),
	}, nil
}

func (server *Server) InitServer() {
	//gob.Register(make(map[uuid.UUID]*Terminal))
	s := server.s

	s.Static("/static", "./static")

	wsUpgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	s.POST("/api/session/new", func(c *gin.Context) {
		session := sessions.Default(c)

		if session.Get("loggedin") == true {
			c.JSON(400, modal.SendErrorMessage("Already logged in"))
			return
		}

		// TODO - Authenticate
		session.Set("loggedin", true)
		session.Set("terms", []string{})
		sErr := session.Save()

		if sErr != nil {
			log.Println(sErr)
			c.JSON(500, modal.SendErrorMessage(modal.SessionError))
			return
		}

		c.JSON(200, modal.SendSuccessMessage("Session created"))
	})

	s.POST("/api/term/new", func(c *gin.Context) {
		session := isAuthorized(c)
		if session == nil {
			return
		}

		// Get parameters
		var params TerminalParams

		if err := c.BindJSON(&params); err != nil {
			log.Println(params)
			c.JSON(500, modal.SendError(err))
			return
		}

		uTids := session.Get("terms").([]string)
		tId := server.tman.NewTerminal(params.Title, params.Params...)

		uTids = append(uTids, tId)
		session.Set("terms", uTids)

		sErr := session.Save()

		if sErr != nil {
			log.Println(sErr)
			c.JSON(500, modal.SendErrorMessage(modal.SessionError))
			return
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": modal.Success,
			"uuid":    tId,
		})
	})

	s.GET("/api/term/list", func(c *gin.Context) {
		session := isAuthorized(c)
		if session == nil {
			return
		}

		uTids := session.Get("terms").([]string)

		c.JSON(200, modal.ActiveTerminals(uTids))
	})

	s.GET("/api/term/:id/start", func(c *gin.Context) {
		id := c.Param("id")

		session := isAuthorized(c)
		if session == nil {
			return
		}

		terms := session.Get("terms").([]string)

		found := false
		for _, val := range terms {
			if val == id {
				found = true
			}
		}

		if !found {
			c.JSON(400, modal.SendErrorMessage("Terminal not found"))
			return
		}

		startErr := server.tman.StartTerminal(id)

		if startErr != nil {
			c.JSON(400, modal.SendError(startErr))
			return
		}

		c.JSON(200, modal.SendSuccess())
	})

	s.GET("/api/term/:id/stop", func(c *gin.Context) {
		id := c.Param("id")

		session := isAuthorized(c)
		if session == nil {
			return
		}

		terms := session.Get("terms").([]string)

		found := false
		for _, val := range terms {
			if val == id {
				found = true
			}
		}

		if !found {
			c.JSON(400, modal.SendErrorMessage("Not found"))
			return
		}

		stopErr := server.tman.StopTerminal(id)

		if stopErr != nil {
			c.JSON(400, modal.SendError(stopErr))
			return
		}

		c.JSON(200, modal.SendSuccess())
	})

	s.GET("/api/term/:id/output", func(c *gin.Context) {
		id := c.Param("id")

		session := isAuthorized(c)
		if session == nil {
			return
		}

		terms := session.Get("terms").([]string)

		found := false
		for _, val := range terms {
			if val == id {
				found = true
			}
		}

		if !found {
			c.JSON(400, modal.SendErrorMessage("Not found"))
			return
		}

		thisTerm, tErr := server.tman.GetTerm(id)

		if tErr != nil {
			log.Println(tErr)
			return
		}
		// make a websocket
		ws, wErr := wsUpgrader.Upgrade(c.Writer, c.Request, nil)

		if wErr != nil {
			c.JSON(400, modal.SendError(wErr))
		}

		defer ws.Close()

		// start websocket
		var com modal.WsCommand
		var success = false

		for {
			if err := ws.ReadJSON(&com); err != nil {
				log.Println(err)
				break
			}

			switch com.Command {
			case modal.OpenSocket:
				success = acknowledgeSocket(&com, ws)
			case modal.GetTerm:
				success = sendTermSocket(&com, ws, thisTerm)
			}

			if !success {
				break
			}
		}

	})
}

// Websocket handlers
func acknowledgeSocket(c *modal.WsCommand, ws *websocket.Conn) bool {
	c.Command = modal.AckSocket
	c.Message = "Welcome"

	if err := ws.WriteJSON(c); err != nil {
		log.Println(err)
		return false
	}
	return true
}

func sendTermSocket(c *modal.WsCommand, ws *websocket.Conn, term *Terminal) bool {
	c.Command = modal.GetTerm

	var wErr error = nil

	// Open LogFile
	ofile := term.GetOutputFilePath()
	w := NewWatcher(ofile)
	w.Start()

	defer w.Close()
	buffer := make([]byte, 1024)

	go func() {
		for {
			nRead, fErr := w.Read(buffer)

			if fErr != nil {
				log.Println(fErr)
				log.Println("Read Error")
				break
			}

			c.Message = string(buffer[:nRead])

			if wErr = ws.WriteJSON(c); wErr != nil {
				log.Println(wErr)
				break
			}
		}
	}()

	// Handle process end event
	go func() {
		var m modal.WsCommand

		m.Command = modal.StopTerm
		m.Message = term.id.String()

		<-term.closeEvent

		if wErr = ws.WriteJSON(m); wErr != nil {
			log.Println(wErr)
		}
	}()

	var res modal.WsCommand
	var cErr error = nil
O:
	for {

		if err := ws.ReadJSON(&res); err != nil {
			log.Println(err)
			cErr = err
			break
		}

		switch res.Command {
		case modal.StopTerm:
			log.Println("Stop")
			sErr := term.Stop()

			if sErr != nil {

				res.Command = "t"
				res.Message = sErr.Error()

				if wErr = ws.WriteJSON(res); wErr != nil {
					log.Println(wErr)
					break
				}
			}
			break O
		case modal.ReciveCommand:
			term.Send(res.Message)
			log.Println(res.Message)
		}
	}

	return /* Terr != nil || */ cErr != nil
}

func (server *Server) Start() error {
	return server.s.Run(":8080")
}

func isAuthorized(c *gin.Context) sessions.Session {
	session := sessions.Default(c)

	if session.Get("loggedin") != true {
		c.JSON(400, modal.SendErrorMessage(modal.NotLoggedIn))
		return nil
	}
	return session
}

func GenRandomBytes(size int) (blk []byte, err error) {
	blk = make([]byte, size)
	_, err = rand.Read(blk)
	return
}

/* func proxy(c *gin.Context) {
	remote, err := url.Parse("")
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	//Define the director func
	//This is a good place to log, for example
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		// req.URL.Path = c.Param("proxyPath")
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
*/
