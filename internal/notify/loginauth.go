package notify

import (
	"errors"
	"net/smtp"
)

// loginAuth implements the SMTP LOGIN mechanism, which net/smtp omits.
type loginAuth struct {
	username string
	password string
}

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:", "username:":
		return []byte(a.username), nil
	case "Password:", "password:":
		return []byte(a.password), nil
	default:
		return nil, errors.New("unexpected SMTP LOGIN challenge: " + string(fromServer))
	}
}
