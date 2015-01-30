package auth

import (
	"io/ioutil"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/golang/glog"
	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/config"
)

var fileBytes []byte

func Auth(rw http.ResponseWriter, req *http.Request) bool {
	if !config.Config.Auth {
		return true
	}
	getToken := req.URL.Query().Get("token")

	if len(getToken) == 0 {
		return false
	}

	if len(fileBytes) == 0 {
		file, err := ioutil.ReadFile(config.Config.Key)
		if err != nil {
			glog.Error("Error reading file")
			return false
		}
		fileBytes = file
	} else {
		glog.Infoln("Reading cached content")
	}
	token, err := jwt.Parse(getToken, func(token *jwt.Token) (interface{}, error) {
		return fileBytes, nil
	})

	if err == nil && token.Valid {
		glog.Infoln("Authentication Successful")

	} else {
		glog.Infoln("Authentication Failed, Invalid Token")
	}

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
