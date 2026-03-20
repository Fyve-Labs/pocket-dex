ARG BASE_IMAGE=alpine

FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.9.0@sha256:c64defb9ed5a91eacb37f96ccc3d4cd72521c4bd18d5442905b95e2226b0e707 AS xx

FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine3.22@sha256:07e91d24f6330432729082bb580983181809e0a48f0f38ecde26868d4568c6ac AS builder

COPY --from=xx / /

RUN apk add --update alpine-sdk ca-certificates openssl clang lld

ARG TARGETPLATFORM

RUN xx-apk --update add musl-dev gcc

# lld has issues building static binaries for ppc so prefer ld for it
RUN [ "$(xx-info arch)" != "ppc64le" ] || XX_CC_PREFER_LINKER=ld xx-clang --setup-target-triple

RUN xx-go --wrap

WORKDIR /usr/local/src/pocket-dex

ARG GOPROXY

ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Propagate pocket-dex version from build args to the build environment
ARG VERSION
RUN make release-binary

RUN xx-verify /go/bin/pocket-dex

FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS stager

RUN mkdir -p /var/pocket-dex

FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS alpine

FROM alpine AS user-setup
RUN addgroup -g 1001 -S pocket-dex && adduser -u 1001 -S -G pocket-dex -D -H -s /sbin/nologin pocket-dex

FROM gcr.io/distroless/static-debian13:nonroot@sha256:f512d819b8f109f2375e8b51d8cfd8aafe81034bc3e319740128b7d7f70d5036 AS distroless

FROM $BASE_IMAGE

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Ensure the pocket-dex user/group exist before setting ownership or switching to them.
COPY --from=user-setup /etc/passwd /etc/passwd
COPY --from=user-setup /etc/group /etc/group

COPY --from=stager --chown=1001:1001 /var/pocket-dex /var/pocket-dex

COPY --from=builder /go/bin/pocket-dex /usr/local/bin/pocket-dex

ENV DATA_PATH=/var/pocket-dex

USER pocket-dex:pocket-dex

ENTRYPOINT ["/usr/local/bin/pocket-dex"]
CMD ["serve", "--http=0.0.0.0:8090"]
