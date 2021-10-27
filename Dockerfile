FROM golang:1.17 as builder

WORKDIR /

COPY go.mod go.sum /

# If go.mod/go.sum don't change the dependencies will be cached
RUN go mod download

COPY . /

# Build the application
RUN CGO_ENABLED=0 go build -o main .

# ---
FROM alpine:3.11.12 as system
LABEL maintainer="Gildas Cherruel <gildas.cherruel@genesys.com>"

# Add CA Certificates and clean
RUN apk update && apk upgrade \
  && apk add ca-certificates \
#  && apk add libcap \
  && rm -rf /var/cache/apk/*

# Creates a harmless user
RUN adduser -D -g '' docker

#set our environment
ARG PROBE_PORT=
ENV PROBE_PORT ${PROBE_PORT}
ARG TRACE_PROBE=
ENV TRACE_PROBE ${TRACE_PROBE}

ARG PORT=8080
ENV PORT ${PORT}

ARG STORAGE_ROOT=/usr/local/storage
ENV STORAGE_ROOT ${STORAGE_ROOT}
ARG STORAGE_URL=https://www.acme.com
ENV STORAGE_URL ${STORAGE_URL}

ARG API_ADMIN_USERNAME=admin
ENV API_ADMIN_USERNAME ${API_ADMIN_USERNAME}
ARG API_TOKEN_SECRET=
ENV API_TOKEN_SECRET ${API_TOKEN_SECRET}
ARG API_TOKEN_EXPIRES=
ENV API_TOKEN_EXPIRES ${API_TOKEN_EXPIRES}

# Preparing the Storage area
RUN mkdir -p ${STORAGE_ROOT}
RUN chown docker ${STORAGE_ROOT}
RUN chmod 700 ${STORAGE_ROOT}

# Expose web port
EXPOSE ${PORT}

# Install application, dependencies first
WORKDIR /usr/local/bin
COPY --from=builder /main /usr/local/bin/cantina
#RUN setcap 'cap_net_bind_service=+ep' ${APP_ROOT}/cantina

USER docker

# CMD is useless here as this image contains only one binary
ENTRYPOINT [ "/usr/local/bin/cantina" ]
