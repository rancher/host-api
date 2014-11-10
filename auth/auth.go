package auth

import (
	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/config"
	jwt "github.com/dgrijalva/jwt-go"
	"net/http"
)

func Auth(rw http.ResponseWriter, req *http.Request) bool {
	if !config.Config.Auth {
		return true
	}

	token, err := jwt.ParseFromRequest(req, func(token *jwt.Token) (interface{}, error) {
		return config.Config.Key, nil
	})

	if err != nil {
		common.CheckError(err, 2)
		return false
	}

	if !token.Valid {
		return false
	}

	if token.Claims["hostUuid"] != config.Config.HostUuid {
		return false
	}

	return true
}
