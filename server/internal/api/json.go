package api

import (
	"encoding/json"
	"net/http"
)

// Envelope 是所有接口统一的响应包裹。成功时 Code=0。
type Envelope struct {
	Code int    `json:"code"`
	Data any    `json:"data"`
	Msg  string `json:"msg"`
}

// WriteOK 写 HTTP 200，code=0 的成功响应。
func WriteOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Envelope{Code: 0, Data: data, Msg: ""})
}

// WriteErr 写指定 HTTP 状态码与业务 code 的错误响应。
func WriteErr(w http.ResponseWriter, httpStatus, code int, msg string) {
	writeJSON(w, httpStatus, Envelope{Code: code, Data: nil, Msg: msg})
}

func writeJSON(w http.ResponseWriter, httpStatus int, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(env)
}
