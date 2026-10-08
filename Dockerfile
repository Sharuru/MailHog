#
# MailTrap Dockerfile
#

FROM golang:1.27.1-alpine

# Build MailTrap from this image's build context so local fixes are included.
RUN apk --no-cache add --virtual build-dependencies \
    git \
    make

WORKDIR /src
COPY . .
RUN go install github.com/go-bindata/go-bindata/go-bindata@v3.1.2 \
  && make build \
  && cp MailTrap /usr/local/bin \
  && apk del --purge build-dependencies

# Add mailtrap user/group with uid/gid 1000.
# This is a workaround for boot2docker issue #581, see
# https://github.com/boot2docker/boot2docker/issues/581
RUN adduser -D -u 1000 mailtrap

USER mailtrap

WORKDIR /home/mailtrap

ENTRYPOINT ["MailTrap"]

# Expose the SMTP and HTTP ports:
EXPOSE 1025 8025
