package clipboard

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/gin-gonic/gin"
)

// Server ...
type Server struct {
	host string
	port string
}

// Config ...
type Config struct {
	Host string
	Port string
}

// New ...
func CreateListener(config *Config) *Server {
	return &Server{
		host: config.Host,
		port: config.Port,
	}
}

// tcp req
func SendRequest(address string, message string) error {
	reader := strings.NewReader(message)
	request, err := http.NewRequest("GET", "http://"+address+"/", reader) //nolint:noctx // deving
	if err != nil {
		fmt.Println(err)
		return breverrors.WrapAndTrace(err)
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	fmt.Println(resp)
	if err != nil {
		fmt.Println(err)
		return breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close() //nolint:errcheck //deving and defer
	return nil
}

// Run ...
func (server *Server) Run() {
	// Starts a new Gin instance with no middle-ware
	r := gin.New()

	r.GET("/", func(c *gin.Context) {
		jsonData, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			// Handle error
			c.String(http.StatusBadRequest, "Can't parse body")
		}
		fmt.Println("Got the clip", string(jsonData))
		c.String(http.StatusOK, "success")
	})
	err := r.Run(server.host + ":" + server.port)
	if err != nil {
		fmt.Println(err)
	}
}
