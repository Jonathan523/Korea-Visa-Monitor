package visa

import "net/http/cookiejar"

func newCookieJar() (*cookiejar.Jar, error) {
	return cookiejar.New(nil)
}
