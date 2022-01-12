package main

import (
	"log"
	"net/http"
	"stargazer/virtual_term/modal"

	"crypto/rand"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Server struct {
	s           *gin.Engine
	activeTerms map[string]*Terminal
	// activeSessions map[string]string
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
		s:           s,
		activeTerms: make(map[string]*Terminal),
	}, nil
}

func (server *Server) InitServer() {
	//gob.Register(make(map[uuid.UUID]*Terminal))
	s := server.s

	s.Static("/static", "./static")
	// Initialize websocket
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

		termuuid := uuid.New()
		terms := session.Get("terms").([]string)

		term := NewTerminal(params.Title, params.Params...)
		server.activeTerms[termuuid.String()] = term

		terms = append(terms, termuuid.String())

		session.Set("terms", terms)

		sErr := session.Save()

		if sErr != nil {
			log.Println(sErr)
			c.JSON(500, modal.SendErrorMessage(modal.SessionError))
			return
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": modal.Success,
			"uuid":    termuuid.String(),
		})
	})

	s.GET("/api/term/list", func(c *gin.Context) {
		session := isAuthorized(c)
		if session == nil {
			return
		}

		terms := session.Get("terms").([]string)

		c.JSON(200, gin.H{
			"error":   false,
			"message": modal.Success,
			"count":   terms,
		})
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

		thisTerm := server.activeTerms[id]

		startErr := thisTerm.StartSingle()

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

		thisTerm := server.activeTerms[id]

		stopErr := thisTerm.Stop()

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

		thisTerm := server.activeTerms[id]

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

	s.GET("/api/testsocket", func(c *gin.Context) {
		// id := c.Param("id")

		// session := isAuthorized(c)
		// if session == nil {
		// 	return
		// }

		// terms := session.Get("terms").([]string)

		// found := false
		// for _, val := range terms {
		// 	if val == id {
		// 		found = true
		// 	}
		// }

		// if !found {
		// 	c.JSON(400, modal.SendErrorMessage("Not found"))
		// 	return
		// }

		// thisTerm := server.activeTerms[id]

		// make a websocket
		ws, wErr := wsUpgrader.Upgrade(c.Writer, c.Request, nil)

		if wErr != nil {
			c.JSON(400, modal.SendError(wErr))
		}

		defer ws.Close()

		// Get terminal output file

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
				// success = sendTermSocket(&com, ws, "test.txt")
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
	buffer := make([]byte, 8)

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

	var res modal.WsCommand
	var cErr error = nil
O:
	for {
		if err := ws.ReadJSON(&res); err != nil {
			log.Println(err)
			// t.Stop()
			cErr = err
			break
		}

		switch res.Command {
		case modal.StopTerm:
			log.Println("Stop")
			term.Stop()
			w.Close()
			// t.Stop()
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
