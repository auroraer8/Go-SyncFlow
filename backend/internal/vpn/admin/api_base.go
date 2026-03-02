package admin

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"go-syncflow/internal/vpn/base"
	"go-syncflow/internal/vpn/pkg/utils"
	"github.com/gorilla/mux"
	"github.com/xlzd/gotp"
)

// Login 登陆接口
func Login(w http.ResponseWriter, r *http.Request) {
	// TODO 调试信息输出
	// hd, _ := httputil.DumpRequest(r, true)
	// fmt.Println("DumpRequest: ", string(hd))

	_ = r.ParseForm()
	adminUser := r.PostFormValue("admin_user")
	adminPass := r.PostFormValue("admin_pass")

	// 启用otp验证
	if base.Cfg.AdminOtp != "" {
		pwd := adminPass
		pl := len(pwd)
		if pl < 6 {
			RespError(w, RespUserOrPassErr)
			base.Error(adminUser, "管理员otp错误")
			return
		}
		// 判断otp信息
		adminPass = pwd[:pl-6]
		otp := pwd[pl-6:]

		totp := gotp.NewDefaultTOTP(base.Cfg.AdminOtp)
		unix := time.Now().Unix()
		verify := totp.Verify(otp, unix)

		if !verify {
			RespError(w, RespUserOrPassErr)
			base.Error(adminUser, "管理员otp错误")
			return
		}
	}

	// 认证错误
	if !(adminUser == base.Cfg.AdminUser &&
		utils.PasswordVerify(adminPass, base.Cfg.AdminPass)) {
		RespError(w, RespUserOrPassErr)
		base.Error(adminUser, "管理员用户名或密码错误")
		return
	}

	// token有效期
	expiresAt := time.Now().Unix() + 3600*3
	jwtData := map[string]interface{}{"admin_user": adminUser}
	tokenString, err := SetJwtData(jwtData, expiresAt)
	if err != nil {
		RespError(w, 1, err)
		return
	}

	data := make(map[string]interface{})
	data["token"] = tokenString
	data["admin_user"] = adminUser
	data["expires_at"] = expiresAt

	ck := &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, ck)

	RespSucess(w, data)
}

func authMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == http.MethodOptions {
			// w.WriteHeader(http.StatusOK)
			// 正式环境不支持 OPTIONS
			w.WriteHeader(http.StatusForbidden)
			return
		}

		route := mux.CurrentRoute(r)
		name := route.GetName()
		// fmt.Println("bb", r.URL.Path, name)
		if utils.InArrStr([]string{"login", "index", "static"}, name) {
			// 不进行鉴权
			next.ServeHTTP(w, r)
			return
		}

		// 进行登陆鉴权
		jwtToken := r.Header.Get("Jwt")
		if jwtToken == "" {
			jwtToken = r.FormValue("jwt")
		}
		if jwtToken == "" {
			cc, err := r.Cookie("jwt")
			if err == nil {
				jwtToken = cc.Value
			}
		}
		data, err := GetJwtData(jwtToken)
		if err != nil || base.Cfg.AdminUser != fmt.Sprint(data["admin_user"]) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// ChangePassword 修改管理员密码
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	oldPass := r.PostFormValue("old_pass")
	newPass := r.PostFormValue("new_pass")
	confirmPass := r.PostFormValue("confirm_pass")

	// 验证原密码
	if !utils.PasswordVerify(oldPass, base.Cfg.AdminPass) {
		RespError(w, 1, "原密码错误")
		return
	}

	// 验证新密码
	if len(newPass) < 6 {
		RespError(w, 1, "新密码长度至少6位")
		return
	}

	if newPass != confirmPass {
		RespError(w, 1, "两次输入的密码不一致")
		return
	}

	// 生成新密码哈希
	hashedPassword, err := utils.PasswordHash(newPass)
	if err != nil {
		RespError(w, 1, "密码加密失败")
		return
	}

	// 更新内存中的密码
	base.Cfg.AdminPass = hashedPassword

	// 更新配置文件
	err = updateConfigFile("admin_pass", hashedPassword)
	if err != nil {
		base.Warn("更新配置文件失败:", err)
	}

	RespSucess(w, "密码修改成功")
}

// updateConfigFile 更新配置文件中的指定字段
func updateConfigFile(key, value string) error {
	configFile := base.Cfg.Conf
	if configFile == "" {
		configFile = "./conf/server.toml"
	}

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = fmt.Sprintf("%s = \"%s\"", key, value)
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, fmt.Sprintf("%s = \"%s\"", key, value))
	}

	// 写回配置文件
	return os.WriteFile(configFile, []byte(strings.Join(lines, "\n")), 0644)
}

func recoverHttp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				base.Error(err, string(stack))
				// http.Error(w, "Internal Server Error", 500)
				RespError(w, 500, "Internal Server Error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
