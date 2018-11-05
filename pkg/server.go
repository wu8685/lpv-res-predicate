package pkg

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
)

type PredicateServer struct {
	cfg *config.Config
}

func NewPredicateServer(cfg *config.Config) *PredicateServer {
	return &PredicateServer{
		cfg: cfg,
	}
}

func (s *PredicateServer) Run() error {
	router := mux.NewRouter()
	for name, init := range restHandlerInits {
		restObj, err := init(s.cfg)
		if err != nil {
			return fmt.Errorf("fail to initialize rest handler %s: %s", name, err)
		}

		for _, rest := range restObj.RestInfos() {
			path := buildPath(name, rest.Path)
			glog.V(0).Infof("Register REST API: %v - %s", path, rest.Methods)
			router.Path(path).Methods(rest.Methods...).HandlerFunc(rest.Handler)
		}
	}

	glog.V(1).Infof("Start informers")
	s.cfg.InformerFactory.Start(wait.NeverStop)

	addr := fmt.Sprintf(":%d", s.cfg.Options.ListenPort)
	glog.V(0).Infof("Start on %s", addr)
	return http.ListenAndServe(addr, router)
}

func buildPath(name, subPath string) string {
	subPath = strings.Trim(subPath, "/")
	if len(subPath) == 0 {
		return "/" + strings.Trim(name, "/")
	}
	return fmt.Sprintf("/%s/%s", strings.Trim(name, "/"), subPath)
}