package auth

import (
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/golang/glog"
	"github.com/gorilla/context"
	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/config"
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

	if config.Config.HostUuidCheck && token.Claims["hostUuid"] != config.Config.HostUuid {
		glog.Infoln("Host UUID mismatch , authentication failed")
		return false
	}

	return true
}

func AuthHttpInterceptor(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()

		if !Auth(w, req) {
			http.Error(w, "Failed authentication", 401)
			return
		}

		router.ServeHTTP(w, req)

		finishTime := time.Now()
		elapsedTime := finishTime.Sub(startTime)

		switch req.Method {
		case "GET":
			// We may not always want to StatusOK, but for the sake of
			// this example we will
			common.LogAccess(w, req, elapsedTime)
		case "POST":
			// here we might use http.StatusCreated
		}

		context.Clear(req)
	})
}
