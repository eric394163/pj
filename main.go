package main

import (
	"database/sql"
	"gopjex/dbcon"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"text/template"
)

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

func DBConnection() (*dbcon.DBConnection, error) {
	dbc := dbcon.NewConnection()
	err := dbc.Open("KSH", "kshi1122@@", "localhost:3306", "gopj")
	if err != nil {
		return nil, err
	}
	return dbc, nil
}

func main() {

	dbc, err := DBConnection()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbc.Close()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.Handle("/login", &templateHandler{filename: "login/login.html"})
	http.Handle("/register", &templateHandler{filename: "register/register.html"})
	http.Handle("/chat", MustAuth(&templateHandler{filename: "chat/chat.html"}))
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
		// 성공한 경우, 원하는 페이지로 리다이렉트하거나 메시지 표시
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	http.HandleFunc("/loginProcess", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		username := r.FormValue("id")
		password := r.FormValue("password")

		// 데이터베이스에서 해당 아이디의 사용자 정보 가져옴
		row := dbc.QueryRow("SELECT password FROM users WHERE username=?", username)

		// 데이터베이스에서 가져온 비밀번호를 저장할 변수
		var storedPassword string

		err := row.Scan(&storedPassword)
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
		// 로그인 성공 후 "auth" 쿠키 설정
		authCookie := &http.Cookie{
			Name:  "auth",
			Value: username, // 예시: 사용자 이름을 값으로 설정. 실제로는 세션 ID나 다른 값을 사용해야 할 수도 있습니다.
			Path:  "/",
		}
		http.SetCookie(w, authCookie)

		// 로그인 성공 시, chat.html로 리다이렉트
		http.Redirect(w, r, "/chat", http.StatusSeeOther)

	})

	log.Println("Starting web Server on : 8180")
	if err := http.ListenAndServe(":8180", nil); err != nil {
		log.Fatal("Listen and Serve :", err)
	}
}
