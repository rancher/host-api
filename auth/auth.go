package auth

import (
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/golang/glog"
	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/config"
	"net/http"
)

func Auth(rw http.ResponseWriter, req *http.Request) bool {
	if !config.Config.Auth {
		return true
	}
	tokenString := req.URL.Query().Get("token")

	if len(tokenString) == 0 {
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return config.Config.ParsedPublicKey, nil
	})
	SetToken(req, token)

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
