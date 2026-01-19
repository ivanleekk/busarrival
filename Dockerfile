FROM golang
LABEL authors="ivanlee"

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -o server
CMD [ "./server" ]

EXPOSE 8080