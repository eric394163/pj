package main

import (
	"database/sql"
	"gopjex/dbcon"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

type authHandler struct {
	next http.Handler
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie("auth")
	if err == http.ErrNoCookie {
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.next.ServeHTTP(w, r)
}
func MustAuth(handler http.Handler) http.Handler {
	return &authHandler{next: handler}
}

func Register(w http.ResponseWriter, r *http.Request, dbc *dbcon.DBConnection) {
	r.ParseForm()
	name := r.FormValue("name")
	email := r.FormValue("email")
	username := r.FormValue("id")
	password := r.FormValue("password")

	stmt, err := dbc.Conn.Prepare("INSERT INTO users (name, email, username, password, is_admin) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		http.Error(w, "Failed to register", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	// 비밀번호 해싱
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		http.Error(w, "Failed to register", http.StatusInternalServerError)
		return
	}
	// 해시된 비밀번호를 데이터베이스에 저장
	_, err = stmt.Exec(name, email, username, string(hashedPassword), 0) //0 은 일반 유저
	if err != nil {
		log.Printf("Failed to insert into database: %v", err)
		http.Error(w, "Failed to register", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func Login(w http.ResponseWriter, r *http.Request, dbc *dbcon.DBConnection) {
	r.ParseForm()
	username := r.FormValue("id")
	password := r.FormValue("password")

	stmt, err := dbc.Conn.Prepare("SELECT password, name, email, is_admin FROM users WHERE username=?")
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	var storedPassword, name, email string
	var isAdmin bool
	err = stmt.QueryRow(username).Scan(&storedPassword, &name, &email, &isAdmin)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid login credentials", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to get user data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
	if err != nil {
		http.Error(w, "Invalid login credentials", http.StatusUnauthorized)
		return
	}

	// 로그인 성공 하면 쿠키 설정

	authCookie := &http.Cookie{
		Name:  "auth",
		Value: username,
		Path:  "/",
	}
	http.SetCookie(w, authCookie)
	nameCookie := &http.Cookie{
		Name:  "name",
		Value: url.QueryEscape(name),
		Path:  "/",
	}
	http.SetCookie(w, nameCookie)
	emailCookie := &http.Cookie{
		Name:  "email",
		Value: url.QueryEscape(email),
		Path:  "/",
	}
	http.SetCookie(w, emailCookie)
	adminCookie := &http.Cookie{
		Name:  "isAdmin",
		Value: strconv.FormatBool(isAdmin), // bool 값을 string으로 변환해서 저장
		Path:  "/",
	}
	http.SetCookie(w, adminCookie)

	// chat.html 리다이렉트
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}
