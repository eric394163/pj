package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"text/template"

	"gopjex/dbcon"

	"github.com/gorilla/websocket"
)

var mainRoom = newRoom()
var ismainRoomRunning = false // 메인채팅방의 실행 유무 확인용 변수
var dbc *dbcon.DBConnection

type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename))) // (templates/ ?)
	})
	t.templ.Execute(w, nil)
}

func DBConnection() (*dbcon.DBConnection, error) {
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
		if !ismainRoomRunning {
			go mainRoom.run(dbc)
			ismainRoomRunning = true
		}
		mainRoom.ServeHTTP(w, r)
	} else {
		MustAuth(&templateHandler{filename: "chat/chat.html"}).ServeHTTP(w, r)
	}
}

func main() {

	//db연결
	var err error
	dbc, err = DBConnection()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbc.Close()

	//핸들러 모음
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.Handle("/login", &templateHandler{filename: "login/login.html"})
	http.Handle("/register", &templateHandler{filename: "register/register.html"})
	http.HandleFunc("/processRegister", func(w http.ResponseWriter, r *http.Request) {
		Register(w, r, dbc)
	})
	http.HandleFunc("/processLogin", func(w http.ResponseWriter, r *http.Request) {
		Login(w, r, dbc)
	})
	http.HandleFunc("/chat", handleChat)

	//서버 실행
	if err := http.ListenAndServe("localhost:8180", nil); err != nil {
		log.Fatal("Listen and Serve :", err)
	}
}
