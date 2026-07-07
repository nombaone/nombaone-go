# Publishing `nombaone-go`

This guide is for the person who ships releases — **you do not need to know Go.**
Go is the easiest of all the SDKs to publish: there is **no registry account, no
token, no 2FA, and nothing to upload.** A published Git tag *is* the release. The
public Go module proxy serves it automatically, and `pkg.go.dev` indexes it.

The package import path is **`github.com/nombaone/nombaone-go`**.

---

## One-time setup (do this once, ever)

1. **Create the GitHub repository** `nombaone-go` under the `nombaone`
   organization, so its URL is exactly:

   ```
   https://github.com/nombaone/nombaone-go
   ```

   The import path must match this URL — do not rename the org or repo.

2. **Push this code to it** (from the SDK folder):

   ```bash
   git remote add origin https://github.com/nombaone/nombaone-go.git
   git push -u origin main
   ```

3. That's it. There is **no account to register**, no publisher to authorize, no
   secret to add. CI (GitHub Actions) is already configured and will run the
   quality gate on every push.

---

## The release ritual (every release)

**Two steps. No tags, no uploads, no tokens, no laptop ceremony.**

1. **Bump the one version line** in [`version.go`](version.go), and add a matching
   section to [`CHANGELOG.md`](CHANGELOG.md):

   ```go
   const Version = "0.1.0"   // → change to "0.1.1", "0.2.0", …
   ```

2. **Merge to `main`.** That's the whole release.

CI does the rest automatically: it runs the full quality gate (format, vet,
staticcheck, tests on Go 1.23 and 1.24), then — only if `version.go` names a
version with no tag yet — creates and pushes the `vX.Y.Z` tag and cuts a GitHub
Release. Within about a minute, `go get github.com/nombaone/nombaone-go@v0.1.0`
works worldwide. **The tag is the release; the Go module proxy serves it.**

> **"Publish only if new" is automatic.** The auto-tag job skips when the tag
> already exists, and Git tags are immutable on the Go proxy — a version
> publishes exactly once. A merge that doesn't bump the version line is a no-op.
> Always bump the version.

> You never run `git tag` by hand. If you ever need to (e.g. re-cut a botched
> release), delete the bad tag first — the proxy caches the first thing it saw.

---

## Post-publish check (clean room — proves the release actually works)

From a brand-new empty folder on any machine (not this working copy):

```bash
mkdir /tmp/nombaone-smoke && cd /tmp/nombaone-smoke
go mod init smoke
go get github.com/nombaone/nombaone-go@v0.1.0     # pulls the PUBLISHED module
cat > main.go <<'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	nombaone "github.com/nombaone/nombaone-go"
)

func main() {
	c, err := nombaone.New(nombaone.WithAPIKey(os.Getenv("NOMBAONE_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	cust, err := c.Customers.Create(context.Background(), nombaone.CustomerCreateParams{
		Email: "smoke@example.com", Name: "Smoke Test",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("created", cust.ID, "mode", cust.Mode)
}
EOF
NOMBAONE_API_KEY=nbo_sandbox_… go run .
# expect: created nbo…cus mode sandbox
```

If that prints a `nbo…cus` id, the published release is good.

---

## What to trust before you ship

Run the exhaustive live check once (needs a sandbox key), and read its verdict:

```bash
NOMBAONE_INTEGRATION=1 NOMBAONE_API_KEY=nbo_sandbox_… \
NOMBAONE_BASE_URL=https://sandbox.api.nombaone.xyz \
go test -run TestIntegrationFullSurface -v ./
```

The last lines print a verdict you can read without knowing Go:

```
VERDICT: N method checks across 15 namespaces | ok X | expected-errors Y | DEFECTS 0
```

**Ship only when `DEFECTS 0`.** "expected-errors" are methods that correctly
returned a typed business error (e.g. no settlement subaccount configured) —
those are the SDK working, not failing.
