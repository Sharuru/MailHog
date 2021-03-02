VERSION=1.0.0

build:
	go mod vendor
	rm customize/MailHog-UI/assets/assets.go
	make -C customize/MailHog-UI/ bindata
	cp customize/MailHog-UI/assets/assets.go vendor/github.com/mailhog/MailHog-UI/assets/assets.go
	cp customize/MailHog-Server/config/config.go vendor/github.com/mailhog/MailHog-Server/config/config.go
	go build
