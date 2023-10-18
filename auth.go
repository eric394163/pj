package main

import (
	"database/sql"
	"fmt"
	"gopjex/dbcon"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

type authHandler struct {
	next http.Handler
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil {
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	tokenString := cookie.Value
	token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if jwt.GetSigningMethod("HS256") != token.Method {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method)
		}
		return []byte("981122"), nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		authValue, ok := claims["auth"]
		if !ok || authValue != "authorized" {
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
	} else {
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusTemporaryRedirect)
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
	userID := r.FormValue("id")
	password := r.FormValue("password")

	stmt, err := dbc.Conn.Prepare("INSERT INTO users (name, email, userID, password, is_admin) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		http.Error(w, "회원가입 실패", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	// 비밀번호 해싱
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		http.Error(w, "회원가입 실패", http.StatusInternalServerError)
		return
	}
	// 해시된 비밀번호를 데이터베이스에 저장
	_, err = stmt.Exec(name, email, userID, string(hashedPassword), 0) //0 은 일반 유저
	if err != nil {
		log.Printf("Failed to insert into database: %v", err)
		http.Error(w, "회원가입 실패", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func createToken(userID string, email string, isAdmin bool) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["userID"] = userID
	claims["email"] = email
	claims["isAdmin"] = isAdmin
	claims["auth"] = "authorized"
	claims["exp"] = time.Now().Add(time.Hour * 2).Unix()

	t, err := token.SignedString([]byte("1102"))
	if err != nil {
		return "", err
	}
	return t, nil
}

func Login(w http.ResponseWriter, r *http.Request, dbc *dbcon.DBConnection) {
	r.ParseForm()
	userID := r.FormValue("id")
	password := r.FormValue("password")

	stmt, err := dbc.Conn.Prepare("SELECT password, name, email, is_admin FROM users WHERE userID=?")
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	var storedPassword, name, email string
	var isAdmin bool
	err = stmt.QueryRow(userID).Scan(&storedPassword, &name, &email, &isAdmin)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "로그인 실패", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to get user data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
	if err != nil {
		http.Error(w, "로그인 실패", http.StatusUnauthorized)
		return
	}

	// 로그인 성공 하면 토큰 설정

	tokenString, err := createToken(userID, email, isAdmin)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	authCookie := &http.Cookie{
		Name:  "auth",
		Value: userID,
		Path:  "/",
	}
	http.SetCookie(w, authCookie)

	// 토큰을 쿠키에 저장
	http.SetCookie(w, &http.Cookie{
		Name:  "token",
		Value: tokenString,
		Path:  "/",
	})
	// chat.html 리다이렉트
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}
