package handler

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"

	"go-syncflow/internal/vpn/admin"
	"go-syncflow/internal/vpn/dbdata"
)

//go:embed download_page.html
var defaultDownloadPage string

func LinkHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Del("X-Aggregate-Auth")

	connection := strings.ToLower(r.Header.Get("Connection"))
	userAgent := strings.ToLower(r.UserAgent())
	if connection == "close" && (strings.Contains(userAgent, "anyconnect") || strings.Contains(userAgent, "openconnect")) {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	index := &dbdata.SettingOther{}
	if err := dbdata.SettingGet(index); err != nil {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, defaultDownloadPage)
		return
	}

	if index.Homecode != http.StatusOK {
		w.WriteHeader(index.Homecode)
		return
	}
	w.WriteHeader(http.StatusOK)

	if index.Homeindex == "" {
		fmt.Fprintln(w, defaultDownloadPage)
	} else {
		fmt.Fprintln(w, index.Homeindex)
	}
}

func LinkOtpQr(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")

	_ = r.ParseForm()
	idS := r.FormValue("id")
	jwtToken := r.FormValue("jwt")
	data, err := admin.GetJwtData(jwtToken)
	if err != nil || idS != fmt.Sprint(data["id"]) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	admin.UserOtpQr(w, r)
}
