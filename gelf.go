package gelf

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gliderlabs/logspout/router"
)

var hostname string

func init() {
	hostname = os.Getenv("LOGSPOUT_HOSTNAME")
	if hostname == "" {
		if _, err := os.Stat("/opt/dockerhostname"); os.IsNotExist(err) {
			hostname, _ = os.Hostname()
		}
		b, err := ioutil.ReadFile("/opt/dockerhostname")
		if err != nil {
			hostname, _ = os.Hostname()
		} else {
			hostname = string(b)
		}

	}
	router.AdapterFactories.Register(NewGelfAdapter, "gelf")
}

// GelfAdapter is an adapter that streams UDP JSON to Graylog
type GelfAdapter struct {
	conn  net.Conn
	route *router.Route
}

// NewGelfAdapter creates a GelfAdapter with UDP as the default transport.
func NewGelfAdapter(route *router.Route) (router.LogAdapter, error) {
	transport, found := router.AdapterTransports.Lookup(route.AdapterTransport("udp"))
	if !found {
		return nil, errors.New("unable to find adapter: " + route.Adapter)
	}

	conn, err := transport.Dial(route.Address, route.Options)
	if err != nil {
		return nil, err
	}

	return &GelfAdapter{
		route: route,
		conn:  conn,
	}, nil
}

// Stream implements the router.LogAdapter interface.
func (a *GelfAdapter) Stream(logstream chan *router.Message) {
	for m := range logstream {

		msg := GelfMessage{
			Version:          "1.1",
			Host:             hostname, // Running as a container cannot discover the Docker Hostname
			ShortMessage:     m.Data,
			Timestamp:        m.Time.Unix(),
			ContainerId:      m.Container.ID,
			ContainerName:    strings.TrimLeft(m.Container.Name, "/"),
			ContainerCmd:     strings.Join([]string{strings.Join(m.Container.Config.Entrypoint, " "), strings.Join(m.Container.Config.Cmd, " ")}, " "),
			ImageId:          m.Container.Image,
			ImageName:        m.Container.Config.Image,
			ContainerCreated: m.Container.Created.Format(time.RFC3339Nano),
		}

		if m.Source == "stdout" {
			msg.Level = 3
		}

		if m.Source == "stderr" {
			msg.Level = 6
		}

		js, err := json.Marshal(msg)
		if err != nil {
			log.Println("Graylog:", err)
			continue
		}
		_, err = a.conn.Write(js)
		if err != nil {
			log.Println("Graylog:", err)
			continue
		}
	}
}

type GelfMessage struct {
	Version      string `json:"version"`
	Host         string `json:"host,omitempty"`
	ShortMessage string `json:"short_message"`
	FullMessage  string `json:"message,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	Level        int    `json:"level,omitempty"`

	ImageId          string `json:"image_id,omitempty"`
	ImageName        string `json:"image_name,omitempty"`
	ContainerId      string `json:"container_id,omitempty"`
	ContainerName    string `json:"container_name,omitempty"`
	ContainerCmd     string `json:"command,omitempty"`
	ContainerCreated string `json:"created,omitempty"`
}
