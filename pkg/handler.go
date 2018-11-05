package pkg

import (
	"fmt"
	"net/http"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
)

type HandlerInit func(*config.Config) (Handler, error)

type Handler interface {
	RestInfos() []RestInfo
}

type RestInfo struct {
	Path    string
	Methods []string
	Handler func(http.ResponseWriter, *http.Request)
}

func RegisterHandlerInit(name string, handlerInit HandlerInit) {
	if _, exist := restHandlerInits[name]; exist {
		panic(fmt.Sprintf("handler init %s already exist", name))
	}

	restHandlerInits[name] = handlerInit
}

var restHandlerInits = map[string]HandlerInit{}