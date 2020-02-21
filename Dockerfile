FROM golang:1.13 as builder

WORKDIR /

COPY go.mod go.sum /

# If go.mod/go.sum don't change the dependencies will be cached
RUN go mod download

COPY . /

# Build the application
RUN CGO_ENABLED=0 go build -o main .

# ---
FROM alpine:3.9 as system

# Add CA Certificates and clean
RUN apk update && apk upgrade \
  && apk add ca-certificates \
#  && apk add libcap \
  && rm -rf /var/cache/apk/*

# Creates a harmless user
RUN adduser -D -g '' docker

#RUN setcap 'cap_net_bind_service=+ep' ${APP_ROOT}/purecloud_connector

# ---
FROM scratch
LABEL maintainer="Gildas Cherruel <gildas.cherruel@genesys.com>"

ENV APP_ROOT /

#set our environment
ARG PROBE_PORT=
ENV PROBE_PORT ${PROBE_PORT}
ARG TRACE_PROBE=
ENV TRACE_PROBE ${TRACE_PROBE}

ARG PORT=8080
ENV PORT ${PORT}

ARG API_ADMIN_USERNAME=admin
ENV API_ADMIN_USERNAME ${API_ADMIN_USERNAME}
ARG API_TOKEN_SECRET=
ENV API_TOKEN_SECRET ${API_TOKEN_SECRET}
ARG API_TOKEN_EXPIRES=
ENV API_TOKEN_EXPIRES ${API_TOKEN_EXPIRES}

# Add CA Certificates  and passwd from the builder
COPY --from=system /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=system /etc/passwd                        /etc/passwd

# Expose web port
EXPOSE ${PORT}

# Install application, dependencies first
WORKDIR ${APP_ROOT}
COPY --from=builder /main ${APP_ROOT}/cantina

USER docker

# CMD is useless here as this image contains only one binary
ENTRYPOINT [ "/cantina" ]
