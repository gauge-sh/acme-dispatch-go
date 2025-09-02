package webui

import (
	"fmt"
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type userFormPage struct {
	Target *core.User
	Error  string
	Fields map[string]string
}

func (a *App) userList(w http.ResponseWriter, r *http.Request) {
	page := listPage(r)
	users, total, err := a.store.ListUsers(store.UserFilter{}, page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "users", struct {
		Users []core.User
		Pager pager
	}{users, makePager(r, page, total)})
}

func (a *App) userNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "user_form", userFormPage{
		Target: &core.User{Role: core.RoleDispatcher, IsActive: true},
	})
}

func userFromForm(r *http.Request, u *core.User) {
	u.Email = formStr(r, "email")
	u.Name = formStr(r, "name")
	u.Role = core.Role(formStr(r, "role"))
	u.IsActive = r.FormValue("is_active") == "on"
}

// checkPassword keeps a floor under interactive logins; the length rule is a
// web-side policy, not part of the domain.
func checkPassword(password string, required bool) error {
	if password == "" && !required {
		return nil
	}
	if len(password) < 8 {
		return core.InvalidFields("password must be at least 8 characters",
			map[string]string{"password": "must be at least 8 characters"})
	}
	return nil
}

func (a *App) userCreate(w http.ResponseWriter, r *http.Request) {
	var user core.User
	userFromForm(r, &user)
	password := r.FormValue("password")
	err := checkPassword(password, true)
	if err == nil {
		err = a.store.CreateUser(&user, password)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			a.render(w, r, http.StatusUnprocessableEntity, "user_form",
				userFormPage{Target: &user, Error: message, Fields: fields})
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "User created.")
	http.Redirect(w, r, fmt.Sprintf("/users/%d", user.ID), http.StatusSeeOther)
}

func (a *App) userDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, err := a.store.UserByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tokens, err := a.store.TokensForUser(id)
	if err != nil {
		a.serverError(w, err)
		return
	}
	// A token issued by the previous request rides in on the session; it is
	// rendered exactly once and never stored in the clear.
	session, _ := a.sessions.Get(r, sessionName)
	plaintext, _ := session.Values["new_token"].(string)
	if plaintext != "" {
		delete(session.Values, "new_token")
		if err := session.Save(r, w); err != nil {
			a.serverError(w, err)
			return
		}
	}
	a.render(w, r, http.StatusOK, "user_detail", struct {
		Target   *core.User
		Tokens   []core.APIToken
		NewToken string
	}{user, tokens, plaintext})
}

func (a *App) userEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, err := a.store.UserByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, http.StatusOK, "user_form", userFormPage{Target: user})
}

func (a *App) userUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, err := a.store.UserByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	userFromForm(r, user)
	password := r.FormValue("password")
	err = checkPassword(password, false)
	if err == nil {
		err = a.store.UpdateUser(currentUser(r).ID, user)
	}
	if err == nil && password != "" {
		err = a.store.SetUserPassword(id, password)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			a.render(w, r, http.StatusUnprocessableEntity, "user_form",
				userFormPage{Target: user, Error: message, Fields: fields})
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "User updated.")
	http.Redirect(w, r, fmt.Sprintf("/users/%d", id), http.StatusSeeOther)
}

func (a *App) userDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.DeleteUser(id); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/users/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "User deleted.")
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (a *App) tokenIssue(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	plaintext, _, err := a.store.IssueToken(id, formStr(r, "name"))
	if err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/users/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	session, _ := a.sessions.Get(r, sessionName)
	session.Values["new_token"] = plaintext
	if err := session.Save(r, w); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/users/%d", id), http.StatusSeeOther)
}

func (a *App) tokenRevoke(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	fields := map[string]string{}
	userID := formInt(r, "user_id", fields)
	if err := a.store.DeleteToken(id); err != nil {
		if _, _, ok := formError(err); ok {
			http.NotFound(w, r)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Token revoked.")
	http.Redirect(w, r, fmt.Sprintf("/users/%d", userID), http.StatusSeeOther)
}
