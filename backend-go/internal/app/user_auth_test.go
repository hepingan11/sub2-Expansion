package app

import "testing"

func TestAttachAdminSessionOnlyForSub2APIAdmins(t *testing.T) {
	app := &App{cfg: Config{AuthSecret: "test-secret", AuthTokenTTLHours: 1}}
	admin := Sub2APIUserLoginData{
		AccessToken: "sub2api-access-token",
		User:        []byte(`{"email":"admin@example.com","role":"admin"}`),
	}
	app.attachAdminSession(&admin)
	if !admin.IsAdmin || admin.AdminToken == "" || !app.verifyToken(admin.AdminToken) {
		t.Fatalf("admin session = %#v, want a valid admin token", admin)
	}

	user := Sub2APIUserLoginData{
		AccessToken: "sub2api-access-token",
		User:        []byte(`{"email":"user@example.com","role":"user"}`),
	}
	app.attachAdminSession(&user)
	if user.IsAdmin || user.AdminToken != "" {
		t.Fatalf("user session = %#v, want no admin token", user)
	}
}
