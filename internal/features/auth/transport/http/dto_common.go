package http

type registerReq struct {
	Name		string	`json:"name"`
	Role		string	`json:"role"`
	ManagerCode	string	`json:"manager_code"`
}

type loginReq struct {
	AccessKey string `json:"access_key"`
}

type errorRes struct {
	Error string `json:"error"`
}
