# Eneba-style marketplace

Go (stdlib only) REST backend + React 18 frontend (served by the Go server, no build step).

## Run
    go run .        # http://localhost:8080

## Deploy (Render free tier)
Push to GitHub -> Render "New Web Service" -> Runtime: Docker -> Deploy. PORT is read from env.

## Features
- Store: filters (platform, region, price), sorting, name search
- Home: banner / best sellers / deals, each from its own endpoint (/api/home/*)
- Search: header search on every page; typo/partial tolerant (Levenshtein + substring)
- Product page: details + offers from 3 sellers, cheapest highlighted
- Cart (localStorage) -> sign-in-gated checkout -> simulated payment (test card 4242 4242 4242 4242) -> key shown + order history
- Auth: register/login, token-based; /api/checkout and /api/orders require it

## Scope decisions (and why)
- In-memory data and users: no DB setup, resets on restart. Swap for Postgres/SQLite behind the same handlers.
- Salted SHA-256 passwords: stdlib only; production would use bcrypt/argon2.
- Plain JS React via CDN + Babel in browser: zero build step; production would use Vite + TypeScript.
- Cover art is a generated gradient (no copyrighted images); styling approximates Eneba's dark purple + yellow CTA.
