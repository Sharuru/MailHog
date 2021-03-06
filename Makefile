VERSION=1.0.0

build:
	go mod vendor
	rm -f customize/MailHog-UI/assets/assets.go
	make -C customize/MailHog-UI/ bindata
	cp customize/MailHog-UI/assets/assets.go vendor/github.com/mailhog/MailHog-UI/assets/assets.go
	cp customize/MailHog-Server/config/config.go vendor/github.com/mailhog/MailHog-Server/config/config.go
	cp customize/data/message.go vendor/github.com/mailhog/data/message.go
	cp customize/storage/maildir.go vendor/github.com/mailhog/storage/maildir.go
	go build -o MailTrap
	