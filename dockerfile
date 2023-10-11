# 베이스 이미지 설정
FROM golang:1.18

# 작업 디렉터리 설정
WORKDIR /app

# 의존성 파일 복사 및 설치
COPY go.mod go.sum ./
RUN go mod download

# 소스코드 복사
COPY . .

# 빌드
RUN go build -o pj .

# 실행
CMD ["./pj"]
