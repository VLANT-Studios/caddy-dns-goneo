# Caddy DNS Provider for Goneo API

This is a custom Caddy DNS provider module that uses the `goneo_api` to automatically manage DNS records (such as ACME TXT challenges) for TLS certificate issuance.

## Building

Since Caddy uses standard Go modules, you can build a custom Caddy binary with this plugin using [xcaddy](https://github.com/caddyserver/xcaddy):

```bash
xcaddy build \
    --with github.com/user/caddy-dns-goneo=./caddy_module
```

## Configuration

In your `Caddyfile`, configure the TLS directive to use this DNS provider. You can specify the `host` (the URL where `goneo_api` is running) and your `api_key`.

```caddyfile
example.com {
    tls {
        dns goneo {
            host "http://localhost:3000"
            api_key "YOUR_SECRET_API_KEY"
        }
    }

    respond "Hello, world!"
}
```

### JSON Configuration

If you use Caddy's JSON config, the module ID is `dns.providers.goneo`:

```json
{
  "module": "goneo",
  "host": "http://localhost:3000",
  "api_key": "YOUR_SECRET_API_KEY"
}
```

## How it works

This module implements the `libdns` interfaces (`RecordGetter`, `RecordAppender`, `RecordSetter`, `RecordDeleter`) by making REST API calls to the `goneo_api` endpoints. When Caddy requests a TLS certificate for your domain, it uses this module to temporarily append the required `_acme-challenge` TXT records to your Goneo DNS, waits for propagation, and then cleans them up.
