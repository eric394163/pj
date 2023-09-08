package dbcon

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

type DBConnection struct {
	Conn *sql.DB
}

// NewConnection 함수는 새로운 데이터베이스 연결 생섬함
func NewConnection() *DBConnection {
	return &DBConnection{}
}

// 데이터베이스에 연결
func (dbc *DBConnection) Open(username, password, host, databasename string) error {
	//데이터베이스 연결 설정
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", username, password, host, databasename)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	// 데이터베이스 연결 확인
	if err := db.Ping(); err != nil {
		return err
	}

	dbc.Conn = db
	return nil
}

// 데이터베이스 연결을 닫기
func (dbc *DBConnection) Close() {
	if dbc.Conn != nil {
		dbc.Conn.Close()
	}
}

// Query 함수는 주어진 쿼리를 실행하고 결과 행들을 반환합니다.
func (dbc *DBConnection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if dbc.Conn == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	return dbc.Conn.Query(query, args...)
}

func (dbc *DBConnection) QueryRow(query string, args ...interface{}) *sql.Row {
	if dbc.Conn == nil {
		log.Fatalf("database connection is not initialized")
		return nil
	}
	return dbc.Conn.QueryRow(query, args...)
}
