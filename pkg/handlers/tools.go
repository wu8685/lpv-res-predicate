package handlers

import (
	"net/http"
	"encoding/json"
	"io"

	"github.com/golang/glog"
)

func ReadRequestBody(r *http.Request, body interface{}) (err error) {
	if err = json.NewDecoder(r.Body).Decode(body); err == io.EOF {
		err = nil
		return
	}

	return
}

func WriteResponse(rw http.ResponseWriter, code int, result interface{}) {
	entityStr := []byte{}
	var innerErr error
	if result != nil {
		entityStr, innerErr = json.Marshal(result)
		if innerErr != nil {
			glog.Errorf("fail to marshal response entity: %s", innerErr)
			return
		}
	}
	glog.V(3).Infof("Predicate with response: %s", string(entityStr))

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)
	rw.Write(entityStr)
}
