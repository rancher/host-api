package auth

import (
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/golang/glog"
	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/config"
)

func Auth(rw http.ResponseWriter, req *http.Request) bool {
	if !config.Config.Auth {
		return true
	}
	getToken := req.URL.Query().Get("token")

	if len(getToken) == 0 {
		return false
	}

	token, err := jwt.Parse(getToken, func(token *jwt.Token) (interface{}, error) {
		return config.FileBytes, nil
	})

	if err != nil {
		common.CheckError(err, 2)
		return false
	}

	if !token.Valid {
		return false
	}

	if token.Claims["hostUuid"] != config.Config.HostUuid {
		glog.Infoln("Host UUID mismatch , authentication failed")
		return false
	}

	return true
}
