# Caddy DNS Provider for Goneo API

A custom Caddy DNS provider module for `goneo_api` to automatically manage DNS records (e.g., for ACME TXT challenges).

## Build

Use [xcaddy](https://github.com/caddyserver/xcaddy) to build Caddy with this provider:

```sh
xcaddy build --with github.com/VLANT-Studios/caddy-dns-goneo
```

## Configuration

### Caddyfile

```caddyfile
example.com {
    tls {
        dns goneo {
            host "http://localhost:3000"
            api_key "YOUR_SECRET_API_KEY"
        }
    }
}
```

### JSON

```json
{
  "module": "goneo",
  "host": "http://localhost:3000",
  "api_key": "YOUR_SECRET_API_KEY"
}
```
