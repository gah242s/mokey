package server

import (
	"errors"
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo-contrib/session"
	log "github.com/sirupsen/logrus"
	"github.com/ubccr/goipa"
)

func (h *Handler) checkUser(uid string) (*ipa.UserRecord, error) {
	if len(uid) == 0 {
		return nil, errors.New("Please provide a username")
	}

	userRec, err := h.client.UserShow(uid)
	if err != nil {
		log.WithFields(log.Fields{
			"uid":              uid,
			"ipa_client_error": err,
		}).Error("failed to check user")
		return nil, errors.New("Invalid login")
	}

	return userRec, nil
}

func (h *Handler) tryAuth(uid, password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("Please provide a password")
	}

	client := ipa.NewDefaultClient()

	err := client.Login(uid, password)
	if err != nil {
		log.WithFields(log.Fields{
			"uid":              uid,
			"ipa_client_error": err,
		}).Error("tryauth: failed login attempt")
		return "", errors.New("Invalid login")
	}

	// Ping to get sessionID for later use
	_, err = client.Ping()
	if err != nil {
		return "", errors.New("Error contacting FreeIPA")
	}

	return client.SessionID(), nil
}

func (h *Handler) Login(c echo.Context) error {
	message := ""
	sess, _ := session.Get(CookieKeySession, c)

	if c.Request().Method == "POST" {
		uid := c.FormValue("uid")
		password := c.FormValue("password")

		sid, err := h.tryAuth(uid, password)
		if err != nil {
			message = err.Error()
		} else {
			sess.Values[CookieKeyUser] = uid
			sess.Values[CookieKeySID] = sid
			sess.Values[CookieKeyAuthenticated] = true

			location := "/"
			wyaf := sess.Values[CookieKeyWYAF]
			if _, ok := wyaf.(string); ok {
				location = wyaf.(string)
			}
			delete(sess.Values, CookieKeyWYAF)

			sess.Save(c.Request(), c.Response())

			return c.Redirect(http.StatusFound, location)
		}
	}

	vars := map[string]interface{}{
		"csrf":    c.Get("csrf").(string),
		"message": message}

	return c.Render(http.StatusOK, "login.html", vars)
}

func (h *Handler) Logout(c echo.Context) error {
	logout(c)
	return c.Redirect(http.StatusFound, "/auth/login")
}

func logout(c echo.Context) {
	sess, _ := session.Get(CookieKeySession, c)
	delete(sess.Values, CookieKeySID)
	delete(sess.Values, CookieKeyUser)
	delete(sess.Values, CookieKeyAuthenticated)
	delete(sess.Values, CookieKeyWYAF)
	sess.Options.MaxAge = -1

	sess.Save(c.Request(), c.Response())
}
