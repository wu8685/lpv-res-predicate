package health

import (
	"net/http"

	"github.com/golang/glog"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
	"github.com/wu8685/lpv-res-predicate/pkg"
)

func init() {
	pkg.RegisterHandlerInit("health", NewHealthChecker)
}

type HealthChecker struct {
}

func NewHealthChecker(cfg *config.Config) (pkg.Handler, error) {
	return &HealthChecker{}, nil
}

func (h *HealthChecker) RestInfos() []pkg.RestInfo {
	return []pkg.RestInfo{
		{
			"/",
			[]string{"GET"},
			h.health,
		},
	}
}

func (h *HealthChecker) health(w http.ResponseWriter, r *http.Request) {
	glog.V(2).Infof("Handle health check")
	w.Write([]byte("OK"))
}