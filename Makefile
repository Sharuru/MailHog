VERSION=1.0.0

build:
	go mod vendor
	rm customize/MailHog-UI/assets/assets.go
	go-bindata -o customize/MailHog-UI/assets/assets.go -pkg assets customize/MailHog-UI/assets/...
	cp customize/MailHog-UI/assets/assets.go vendor/github.com/mailhog/MailHog-UI/assets/assets.go
	go build
