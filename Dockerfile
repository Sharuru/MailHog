#
# MailTrap Dockerfile
#

FROM golang:alpine

# Build MailTrap:
RUN apk --no-cache add --virtual build-dependencies \
    git \
    make \
  && git clone https://github.com/Sharuru/MailHog \
  && cd MailHog \
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
