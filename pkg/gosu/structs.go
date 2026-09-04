package gosu

func createHeader(authType, authValue string) string {
	return authType + " " + authValue
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
