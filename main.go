package main

import (
	"database/sql"
	"flag"
	"gopjex/dbcon"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"text/template"

	"github.com/gorilla/websocket"
)

var mainRoom = newRoom()

type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename)))
	})
	t.templ.Execute(w, nil)
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		go mainRoom.run()
		mainRoom.ServeHTTP(w, r)
	} else {
		MustAuth(&templateHandler{filename: "chat/chat.html"}).ServeHTTP(w, r)
	}
}

func DBConnection() (*dbcon.DBConnection, error) {
	// 환경 변수에서 값 읽기
	DB_USER := os.Getenv("DB_USER")
	DB_PASS := os.Getenv("DB_PASS")
	DB_HOST := os.Getenv("DB_HOST")
	DB_NAME := os.Getenv("DB_NAME")

	dbc := dbcon.NewConnection()
	err := dbc.Open(DB_USER, DB_PASS, DB_HOST, DB_NAME)
	if err != nil {
		return nil, err
	}
	return dbc, nil
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		go mainRoom.run()
		mainRoom.ServeHTTP(w, r)
	} else {
		MustAuth(&templateHandler{filename: "chat/chat.html"}).ServeHTTP(w, r)
	}
}

func main() {

	go mainRoom.run()

	var addr = flag.String("addr", ":8180", "The addr of the application.")
	flag.Parse()

	dbc, err := DBConnection()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbc.Close()
	http.HandleFunc("/chat", handleChat)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.Handle("/login", &templateHandler{filename: "login/login.html"})
	http.Handle("/register", &templateHandler{filename: "register/register.html"})
	http.HandleFunc("/processRegister", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		name := r.FormValue("name")
		email := r.FormValue("email")
		username := r.FormValue("id")
		password := r.FormValue("password")
		_, err := dbc.Query("INSERT INTO users (name, email, username, password) VALUES (?, ?, ?, ?)", name, email, username, password)
		if err != nil {
			log.Printf("Failed to insert into database: %v", err)
			http.Error(w, "Failed to register", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	http.HandleFunc("/loginProcess", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		username := r.FormValue("id")
		password := r.FormValue("password")

		// 데이터베이스에서 해당 아이디의 사용자 정보 가져옴
		row := dbc.QueryRow("SELECT password, name, email FROM users WHERE username=?", username)

		// 데이터베이스에서 가져온 데이터를 저장할 변수
		var storedPassword, name, email string

		err := row.Scan(&storedPassword, &name, &email)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Invalid login credentials", http.StatusUnauthorized)
				return
			}
			log.Printf("Failed to get user data: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if password != storedPassword {
			http.Error(w, "Invalid login credentials", http.StatusUnauthorized)
			return
		}
		// 로그인 성공 하면 auth 쿠키 설정
		authCookie := &http.Cookie{
			Name:  "auth",
			Value: username,
			Path:  "/",
		}
		nameCookie := &http.Cookie{
			Name:  "name",
			Value: url.QueryEscape(name),
			Path:  "/",
		}
		emailCookie := &http.Cookie{
			Name:  "email",
			Value: url.QueryEscape(email),
			Path:  "/",
		}
		http.SetCookie(w, nameCookie)
		http.SetCookie(w, emailCookie)
		http.SetCookie(w, authCookie)

		// 로그인 성공 하면 chat.html 리다이렉트
		http.Redirect(w, r, "/chat", http.StatusSeeOther)

	})

	log.Println("Starting web Server on:", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("Listen and Serve :", err)
	}
}
