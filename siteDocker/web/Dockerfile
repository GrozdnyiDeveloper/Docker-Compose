FROM golang:latest

WORKDIR /web

COPY . /web

RUN go build -o web .

EXPOSE 9090

CMD ["./web"]