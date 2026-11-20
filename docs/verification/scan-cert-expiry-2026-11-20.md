# Scan certificate expiry verification

Run on Linux amd64 (cloud container) with Go 1.27.4 and OpenSSL 3.0.13, on
the branch for issue #84, stacked on the branch for #82. Every connection
went to an owned listener on `127.0.0.1:8443` with a disposable `APPDATA`.

## Method

A small fixture accepted TCP connections on loopback, counted every accept,
optionally wrapped the connection in TLS with a generated certificate, and
answered one `GET /`. The non-CGO CLI was built twice, from the parent
commit ("base") and from this change ("new"), and each scanned the fixture:

```sh
lancartographer --cli --range 127.0.0.1/32 --ports 8443 --timeout 300ms --i-am-authorized --json -
```

## Results

| Fixture certificate | Base port record | New port record | Accepts, base / new |
| --- | --- | --- | --- |
| Self-signed, 400 days | open, no title | adds `CertExpires` 2027-12-25, not trusted | 3 / 3 |
| Self-signed, 10 days | open, no title | adds `CertExpires` 2026-11-30, not trusted | 3 / 3 |
| Self-signed, expired 2025-01-01 | not run | `CertExpires` 2025-01-01, not trusted | – / 3 |
| Issued for 127.0.0.1 and trusted through `SSL_CERT_FILE` | open, title `fixture` | adds `CertExpires` 2027-06-08 and `CertTrusted` true | 3 / 3 |
| None (plain HTTP on 8443) | open, no title | identical to base | 3 / 3 |

- The expiry is read from the handshake the title probe already makes. When
  verification fails it comes from the handshake error; verification is
  never turned off and there is no second attempt. The accept count is the
  same before and after in every paired run.
- An untrusted certificate still yields no title and no HTTP request, as
  before. Only the presented expiry is new, and JSON omits `CertTrusted`
  when it is false.
- A port with no certificate serializes exactly as it did before.

```sh
gofmt -l .
go vet ./internal/model ./internal/probe ./internal/session
GOOS=windows go vet ./internal/model ./internal/probe ./internal/session
GOOS=darwin go vet ./internal/model ./internal/probe ./internal/session
go test ./internal/model ./internal/session -run 'Cert|Compare|Report' -count=1
go test ./internal/probe -run HTTP -count=1 -race
```

All passed. The probe test covers the untrusted and trusted handshakes
against an `httptest` TLS server (trusted through a test-only root pool), a
plain HTTP port and a non-TLS answer on an HTTPS request.

## Not exercised

- The native host panel on macOS or Windows. The text it appends comes from
  `model.Port.CertSummary`, which is tested, but the rendered row, its
  truncation and both themes were not looked at. `internal/gui` could not
  be compiled in this container.
- Trust decisions by the macOS or Windows system stores. The trusted case
  here used Go's Linux `SSL_CERT_FILE` override.
- A real device on a LAN, and the packaged app.
