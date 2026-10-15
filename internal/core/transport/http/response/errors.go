package response

import "net/http"

type errorRes struct {
	Error string `json:"error"`
}

func Error(w http.ResponseWriter, code int, msg string) {
	JSON(w, code, errorRes{Error: msg})
}
