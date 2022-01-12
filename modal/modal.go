package modal

import "github.com/gin-gonic/gin"

const (
	LoginSuccess = "Login success"
	NotLoggedIn  = "Not logged in"
	Success      = "Request success"
	SessionError = "Session Error"
)

func SendError(e error) gin.H {
	return gin.H{
		"success": false,
		"message": e.Error(),
	}
}

func SendErrorMessage(e string) gin.H {
	return gin.H{
		"success": false,
		"message": e,
	}
}

func SendSuccess() gin.H {
	return gin.H{
		"success": true,
		"message": Success,
	}
}

func SendSuccessMessage(m string) gin.H {
	return gin.H{
		"success": true,
		"message": m,
	}
}

// WebSocket modal
type WsCommand struct {
	Command string `json:"c"` // command
	Message string `json:"m"` // message
}

const (
	OpenSocket    = "o"
	AckSocket     = "a"
	GetTerm       = "t"
	StopTerm      = "s"
	ReciveCommand = "r"
)
