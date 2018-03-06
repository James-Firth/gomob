FROM golang:1.10

WORKDIR /go/src/github.com/mattmcmurray/gomob

COPY . .

RUN go get -d -v ./...

RUN go install -v ./...

CMD ["gomob"]
